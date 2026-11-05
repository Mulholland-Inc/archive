import Contacts
import CryptoKit
import EventKit
import Foundation
import HealthKit
import Photos
import UIKit

/// What only lives on the phone, pushed to Archive a batch at a time. Each source keeps its place, a HealthKit anchor or a change token,
/// in UserDefaults only after Archive has the batch, so a sync cut short by iOS picks up where it left off.
actor Syncer {
    static let shared = Syncer()
    static let health = HKHealthStore()

    private var running = false
    private var again = false

    func all() async {
        guard Client.shared.signedIn else { return }
        if running {
            again = true
            return
        }
        running = true
        repeat {
            again = false
            for (name, run) in [("health", healthAll), ("photos", photos), ("contacts", contacts), ("events", events)] as [(String, () async throws -> Void)] {
                do {
                    try await run()
                    await Status.shared.synced(name, nil)
                } catch {
                    await Status.shared.synced(name, error)
                }
            }
        } while again
        running = false
    }

    private func pushed(_ stream: String, _ records: [[String: Any]], full: Bool = false, more: Bool = false) async throws {
        try await Client.shared.push(stream, records, full: full, more: more)
        await Status.shared.add(stream, records.count)
    }

    // MARK: Health

    static let quantities: [HKQuantityTypeIdentifier] = [
        .stepCount, .distanceWalkingRunning, .distanceCycling, .flightsClimbed, .activeEnergyBurned, .basalEnergyBurned, .appleExerciseTime, .appleStandTime,
        .heartRate, .restingHeartRate, .walkingHeartRateAverage, .heartRateVariabilitySDNN, .oxygenSaturation, .respiratoryRate, .vo2Max,
        .bodyMass, .height, .bodyMassIndex, .bodyFatPercentage, .bloodPressureSystolic, .bloodPressureDiastolic, .bloodGlucose,
        .dietaryEnergyConsumed, .dietaryWater,
    ]
    static let categories: [HKCategoryTypeIdentifier] = [.sleepAnalysis, .mindfulSession]
    static var types: [HKSampleType] {
        quantities.map { HKQuantityType($0) } + categories.map { HKCategoryType($0) } + [HKWorkoutType.workoutType()]
    }

    /// Wakes the app when Health has new samples, even in the background; called at every launch, as HealthKit wants.
    nonisolated static func observe() {
        guard HKHealthStore.isHealthDataAvailable() else { return }
        for type in types {
            health.execute(HKObserverQuery(sampleType: type, predicate: nil) { _, done, err in
                guard err == nil else { return done() }
                Task {
                    try? await Syncer.shared.health(type)
                    done()
                }
            })
            health.enableBackgroundDelivery(for: type, frequency: type == HKWorkoutType.workoutType() ? .immediate : .hourly) { _, _ in }
        }
    }

    /// Each type on its own, since the user may allow some and not others; it fails only when none synced.
    private func healthAll() async throws {
        var failed: Error?
        var synced = false
        for type in Self.types {
            do {
                try await health(type)
                synced = true
            } catch {
                failed = error
            }
        }
        if !synced, let failed { throw failed }
    }

    private var units: [HKQuantityType: HKUnit] = [:]

    func health(_ type: HKSampleType) async throws {
        guard Client.shared.signedIn else { return }
        if let q = type as? HKQuantityType, units[q] == nil {
            units[q] = try await Self.health.preferredUnits(for: [q])[q]
        }
        let key = "anchor." + type.identifier
        while true {
            let anchor = UserDefaults.standard.data(forKey: key).flatMap { try? NSKeyedUnarchiver.unarchivedObject(ofClass: HKQueryAnchor.self, from: $0) }
            let q = HKAnchoredObjectQueryDescriptor(predicates: [.sample(type: type)], anchor: anchor, limit: 500)
            let got = try await q.result(for: Self.health)
            let records = got.addedSamples.map(record) + got.deletedObjects.map { ["id": $0.uuid.uuidString, "deleted": true] }
            if !records.isEmpty {
                try await pushed("health", records)
            }
            UserDefaults.standard.set(try NSKeyedArchiver.archivedData(withRootObject: got.newAnchor, requiringSecureCoding: true), forKey: key)
            if records.count < 500 { return }
        }
    }

    private func record(_ s: HKSample) -> [String: Any] {
        var d: [String: Any] = ["type": s.sampleType.identifier, "start": s.startDate.iso, "end": s.endDate.iso, "source": s.sourceRevision.source.name]
        var meta: [String: Any] = [:]
        for (k, v) in s.metadata ?? [:] {
            switch v {
            case let v as HKQuantity: meta[k] = v.description
            case let v as Date: meta[k] = v.iso
            case let v as NSNumber: meta[k] = v
            default: meta[k] = "\(v)"
            }
        }
        switch s {
        case let s as HKQuantitySample:
            let unit = units[s.quantityType] ?? HKUnit.count()
            if s.quantity.is(compatibleWith: unit) {
                d["value"] = s.quantity.doubleValue(for: unit)
                d["unit"] = unit.unitString
            }
        case let s as HKCategorySample:
            d["value"] = s.value
            if s.categoryType == HKCategoryType(.sleepAnalysis) {
                meta["stage"] = ["in bed", "asleep", "awake", "core", "deep", "REM"][safe: s.value]
            }
        case let s as HKWorkout:
            d["type"] = "HKWorkoutActivityType" + s.workoutActivityType.name
            if let kcal = s.statistics(for: HKQuantityType(.activeEnergyBurned))?.sumQuantity() {
                d["value"] = kcal.doubleValue(for: .kilocalorie())
                d["unit"] = "kcal"
            }
            meta["seconds"] = s.duration
            if let m = s.statistics(for: HKQuantityType(.distanceWalkingRunning))?.sumQuantity() ?? s.statistics(for: HKQuantityType(.distanceCycling))?.sumQuantity() {
                meta["meters"] = m.doubleValue(for: .meter())
            }
        default: break
        }
        if let device = s.device?.model ?? s.device?.name { d["device"] = device }
        if !meta.isEmpty { d["meta"] = meta }
        return ["id": s.uuid.uuidString, "data": d]
    }

    // MARK: Photos

    private func photos() async throws {
        let status = PHPhotoLibrary.authorizationStatus(for: .readWrite)
        guard status == .authorized || status == .limited else { return }
        let defaults = UserDefaults.standard
        if let t = defaults.data(forKey: "photos.token"), defaults.object(forKey: "photos.listed") == nil,
           let token = try NSKeyedUnarchiver.unarchivedObject(ofClass: PHPersistentChangeToken.self, from: t) {
            do {
                return try await photoChanges(since: token)
            } catch let e as PHPhotosError where e.code == .persistentChangeTokenExpired {
                defaults.removeObject(forKey: "photos.token")
            }
        }
        // The whole library, oldest first, from where the last listing stopped; what changes meanwhile comes after, from the token taken before it.
        if defaults.data(forKey: "photos.token") == nil {
            defaults.set(try NSKeyedArchiver.archivedData(withRootObject: PHPhotoLibrary.shared().currentChangeToken, requiringSecureCoding: true), forKey: "photos.token")
            defaults.set(0, forKey: "photos.listed")
        }
        let opts = PHFetchOptions()
        opts.sortDescriptors = [NSSortDescriptor(key: "creationDate", ascending: true)]
        let assets = PHAsset.fetchAssets(with: opts)
        var done = defaults.integer(forKey: "photos.listed")
        while done < assets.count {
            let batch = assets.objects(at: IndexSet(done ..< min(done + 40, assets.count)))
            try await pushed("photos", await batch.asyncMap(photo))
            done += batch.count
            defaults.set(done, forKey: "photos.listed")
        }
        defaults.removeObject(forKey: "photos.listed")
    }

    private func photoChanges(since token: PHPersistentChangeToken) async throws {
        var ids: Set<String> = [], gone: Set<String> = []
        var last = token
        for change in try PHPhotoLibrary.shared().fetchPersistentChanges(since: token) {
            let d = try change.changeDetails(for: .asset)
            ids.formUnion(d.insertedLocalIdentifiers.union(d.updatedLocalIdentifiers))
            gone.formUnion(d.deletedLocalIdentifiers)
            last = change.changeToken
        }
        let assets = PHAsset.fetchAssets(withLocalIdentifiers: Array(ids.subtracting(gone)), options: nil)
        for start in stride(from: 0, to: assets.count, by: 40) {
            try await pushed("photos", await assets.objects(at: IndexSet(start ..< min(start + 40, assets.count))).asyncMap(photo))
        }
        if !gone.isEmpty {
            try await pushed("photos", gone.map { ["id": $0, "deleted": true] })
        }
        UserDefaults.standard.set(try NSKeyedArchiver.archivedData(withRootObject: last, requiringSecureCoding: true), forKey: "photos.token")
    }

    private func photo(_ a: PHAsset) async -> [String: Any] {
        var d: [String: Any] = ["width": a.pixelWidth, "height": a.pixelHeight, "favorite": a.isFavorite, "kind": a.kind]
        if let t = a.creationDate { d["created"] = t.iso }
        if let t = a.modificationDate { d["modified"] = t.iso }
        if a.mediaType == .video { d["duration"] = a.duration }
        if let l = a.location?.coordinate {
            d["latitude"] = l.latitude
            d["longitude"] = l.longitude
        }
        if let name = PHAssetResource.assetResources(for: a).first?.originalFilename { d["name"] = name }
        if let jpeg = await thumbnail(a) { d["thumbnail"] = jpeg.base64EncodedString() }
        return ["id": a.localIdentifier, "data": d]
    }

    private func thumbnail(_ a: PHAsset) async -> Data? {
        let opts = PHImageRequestOptions()
        opts.deliveryMode = .highQualityFormat
        opts.resizeMode = .fast
        opts.isNetworkAccessAllowed = true
        return await withCheckedContinuation { done in
            PHImageManager.default().requestImage(for: a, targetSize: CGSize(width: 512, height: 512), contentMode: .aspectFit, options: opts) { img, _ in
                done.resume(returning: img?.jpegData(compressionQuality: 0.7))
            }
        }
    }

    // MARK: Contacts

    private static let contactKeys = [CNContactGivenNameKey, CNContactFamilyNameKey, CNContactOrganizationNameKey, CNContactJobTitleKey,
                                      CNContactEmailAddressesKey, CNContactPhoneNumbersKey, CNContactPostalAddressesKey] as [CNKeyDescriptor]

    private func contacts() async throws {
        guard CNContactStore.authorizationStatus(for: .contacts) == .authorized else { return }
        let store = CNContactStore()
        let token = UserDefaults.standard.data(forKey: "contacts.token")
        let h = try ContactsHistory.since(token, in: store, keys: Self.contactKeys)
        var full = false
        var records: [[String: Any]] = []
        for e in h.events {
            switch e {
            case is CNChangeHistoryDropEverythingEvent:
                full = true
                records = []
            case let e as CNChangeHistoryAddContactEvent: records.append(contact(e.contact))
            case let e as CNChangeHistoryUpdateContactEvent: records.append(contact(e.contact))
            case let e as CNChangeHistoryDeleteContactEvent: records.append(["id": e.contactIdentifier, "deleted": true])
            default: break
            }
        }
        if full && records.isEmpty {
            try await pushed("contacts", [], full: true)
        }
        for start in stride(from: 0, to: records.count, by: 200) {
            let end = min(start + 200, records.count)
            try await pushed("contacts", Array(records[start ..< end]), full: full, more: end < records.count)
        }
        UserDefaults.standard.set(h.token, forKey: "contacts.token")
    }

    private func contact(_ c: CNContact) -> [String: Any] {
        var d: [String: Any] = ["given": c.givenName, "family": c.familyName, "organization": c.organizationName, "job": c.jobTitle,
                                "emails": c.emailAddresses.map { $0.value as String }, "phones": c.phoneNumbers.map { $0.value.stringValue }]
        if let a = c.postalAddresses.first?.value {
            d["city"] = a.city
            d["state"] = a.state
            d["country"] = a.country
        }
        return ["id": c.identifier, "data": d]
    }

    // MARK: Calendar

    /// EventKit keeps no change history, so the events from a year back to two ahead are listed whole, when they differ from the last listing.
    private func events() async throws {
        guard EKEventStore.authorizationStatus(for: .event) == .fullAccess else { return }
        let store = EKEventStore()
        let year: TimeInterval = 365 * 24 * 3600
        let found = store.events(matching: store.predicateForEvents(withStart: .now - year, end: .now + 2 * year, calendars: nil))
        let records = found.map(event)
        let sum = Data(SHA256.hash(data: try JSONSerialization.data(withJSONObject: records, options: .sortedKeys))).base64URL
        if UserDefaults.standard.string(forKey: "events.sum") == sum { return }
        if records.isEmpty {
            try await pushed("events", [], full: true)
        }
        for start in stride(from: 0, to: records.count, by: 200) {
            let end = min(start + 200, records.count)
            try await pushed("events", Array(records[start ..< end]), full: true, more: end < records.count)
        }
        UserDefaults.standard.set(sum, forKey: "events.sum")
    }

    private func event(_ e: EKEvent) -> [String: Any] {
        var d: [String: Any] = ["calendar": e.calendar.title, "title": e.title ?? "", "allDay": e.isAllDay, "start": e.startDate.iso, "end": e.endDate.iso,
                                "status": ["none", "confirmed", "tentative", "cancelled"][safe: e.status.rawValue] ?? "none"]
        if let n = e.notes { d["notes"] = n }
        if let l = e.location { d["location"] = l }
        if let u = e.url { d["url"] = u.absoluteString }
        if let t = e.lastModifiedDate { d["modified"] = t.iso }
        if let o = e.organizer { d["organizer"] = ["email": o.email, "name": o.name ?? ""] }
        d["attendees"] = (e.attendees ?? []).map { p -> [String: Any] in
            ["email": p.email, "name": p.name ?? "", "role": p.participantRole == .optional ? "optional" : "required",
             "status": [EKParticipantStatus.accepted: "accepted", .declined: "declined", .tentative: "tentative"][p.participantStatus] ?? "none"]
        }
        return ["id": "\(e.calendar.calendarIdentifier)/\(e.eventIdentifier ?? e.calendarItemIdentifier)/\(e.startDate.iso)", "data": d]
    }
}

extension Date {
    var iso: String { ISO8601DateFormatter().string(from: self) }
}

extension Array {
    subscript(safe i: Int) -> Element? { indices.contains(i) ? self[i] : nil }
}

extension Array where Element == PHAsset {
    func asyncMap<T>(_ f: (PHAsset) async -> T) async -> [T] {
        var out: [T] = []
        for a in self { await out.append(f(a)) }
        return out
    }
}

extension EKParticipant {
    var email: String { url.scheme == "mailto" ? String(url.absoluteString.dropFirst(7)) : "" }
}

extension PHAsset {
    var kind: String {
        if mediaType == .video { return "video" }
        if mediaSubtypes.contains(.photoLive) { return "live" }
        if mediaSubtypes.contains(.photoScreenshot) { return "screenshot" }
        if mediaSubtypes.contains(.photoPanorama) { return "panorama" }
        if mediaSubtypes.contains(.photoDepthEffect) { return "portrait" }
        return "photo"
    }
}

extension HKWorkoutActivityType {
    var name: String {
        switch self {
        case .running: "Running"
        case .walking: "Walking"
        case .cycling: "Cycling"
        case .swimming: "Swimming"
        case .hiking: "Hiking"
        case .yoga: "Yoga"
        case .functionalStrengthTraining: "FunctionalStrengthTraining"
        case .traditionalStrengthTraining: "TraditionalStrengthTraining"
        case .highIntensityIntervalTraining: "HighIntensityIntervalTraining"
        case .rowing: "Rowing"
        case .elliptical: "Elliptical"
        case .dance: "Dance"
        case .pilates: "Pilates"
        case .tennis: "Tennis"
        case .golf: "Golf"
        case .soccer: "Soccer"
        case .basketball: "Basketball"
        default: "Other\(rawValue)"
        }
    }
}
