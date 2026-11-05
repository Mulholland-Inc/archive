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
            let runs: [(String, () async throws -> Void)] = [("health", healthAll), ("photos", photos), ("contacts", contacts), ("events", events),
                                                              ("reminders", reminders), ("places", places), ("wallet", wallet), ("originals", originals)]
            for (name, run) in runs {
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

    func pushed(_ stream: String, _ records: [[String: Any]], full: Bool = false, more: Bool = false) async throws {
        try await Client.shared.push(stream, records, full: full, more: more)
        await Status.shared.add(Status.row[stream] ?? stream, records.count)
    }

    // MARK: Health

    static let quantities: [HKQuantityTypeIdentifier] = [
        .stepCount, .distanceWalkingRunning, .distanceCycling, .flightsClimbed, .activeEnergyBurned, .basalEnergyBurned, .appleExerciseTime, .appleStandTime,
        .heartRate, .restingHeartRate, .walkingHeartRateAverage, .heartRateVariabilitySDNN, .oxygenSaturation, .respiratoryRate, .vo2Max,
        .bodyMass, .height, .bodyMassIndex, .bodyFatPercentage, .bloodPressureSystolic, .bloodPressureDiastolic, .bloodGlucose,
        .dietaryEnergyConsumed, .dietaryWater,
    ]
    /// Sleep and mindful minutes are durations; the rest, like cycle tracking, are events with a level.
    static let categories: [HKCategoryTypeIdentifier] = [
        .sleepAnalysis, .mindfulSession, .menstrualFlow, .intermenstrualBleeding, .ovulationTestResult, .cervicalMucusQuality, .sexualActivity,
        .pregnancyTestResult, .progesteroneTestResult, .contraceptive, .pregnancy, .lactation, .highHeartRateEvent, .lowHeartRateEvent,
        .irregularHeartRhythmEvent, .lowCardioFitnessEvent, .appleWalkingSteadinessEvent, .handwashingEvent, .toothbrushingEvent,
    ]
    static let durations: Set<HKCategoryTypeIdentifier> = [.sleepAnalysis, .mindfulSession]

    /// Every sample type the app reads, each to its stream: workouts, records for what has no number of its own, the rest health.
    static var types: [HKSampleType] {
        var t: [HKSampleType] = quantities.map { HKQuantityType($0) } + categories.map { HKCategoryType($0) }
        t += [HKWorkoutType.workoutType(), HKObjectType.electrocardiogramType(), HKObjectType.audiogramSampleType()]
        if #available(iOS 18, *) { t.append(HKObjectType.stateOfMindType()) }
        if #available(iOS 26, *) { t.append(HKObjectType.medicationDoseEventType()) }
        return t + clinical
    }

    /// Health Records, from the hospitals the user connected in Health; only where the phone has them.
    static var clinical: [HKSampleType] {
        guard health.supportsHealthRecords() else { return [] }
        let ids: [HKClinicalTypeIdentifier] = [.allergyRecord, .clinicalNoteRecord, .conditionRecord, .immunizationRecord, .labResultRecord,
                                               .medicationRecord, .procedureRecord, .vitalSignRecord, .coverageRecord]
        return ids.compactMap { HKObjectType.clinicalType(forIdentifier: $0) }
    }

    /// What the app asks to read: the samples, workouts' routes and the activity rings. Doses come with the medications the user shares, one by one.
    static var reads: Set<HKObjectType> {
        var r = Set<HKObjectType>(types).union([HKSeriesType.workoutRoute(), HKObjectType.activitySummaryType()])
        if #available(iOS 26, *) { r.remove(HKObjectType.medicationDoseEventType()) }
        return r
    }

    static func stream(_ type: HKSampleType) -> String {
        switch type {
        case is HKWorkoutType: "workouts"
        case is HKQuantityType, is HKCategoryType: "health"
        default: "records"
        }
    }

    /// Wakes the app when Health has new samples, even in the background; called at every launch, as HealthKit wants.
    nonisolated static func observe() {
        guard HKHealthStore.isHealthDataAvailable() else { return }
        let made: Set<HKSampleType> = [HKCategoryType(.sleepAnalysis), HKQuantityType(.stepCount), HKQuantityType(.activeEnergyBurned)]
        for type in types {
            health.execute(HKObserverQuery(sampleType: type, predicate: nil) { _, done, err in
                guard err == nil else { return done() }
                Task {
                    try? await Syncer.shared.health(type)
                    if made.contains(type) { try? await Syncer.shared.healthMade() }
                    done()
                }
            })
            health.enableBackgroundDelivery(for: type, frequency: type == HKWorkoutType.workoutType() ? .immediate : .hourly) { _, _ in }
        }
    }

    /// Each type on its own, since the user may allow some and not others; it fails only when none synced. Then the nights, days and medications made of them.
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
        do {
            try await healthMade()
        } catch {
            failed = failed ?? error
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
            // Workouts carry their routes, so fewer go at a time.
            let limit = type is HKWorkoutType ? 20 : 500
            let q = HKAnchoredObjectQueryDescriptor(predicates: [.sample(type: type)], anchor: anchor, limit: limit)
            let got = try await q.result(for: Self.health)
            var records: [[String: Any]] = []
            for s in got.addedSamples { records.append(await record(s)) }
            records += got.deletedObjects.map { ["id": $0.uuid.uuidString, "deleted": true] }
            if !records.isEmpty {
                try await pushed(Self.stream(type), records)
            }
            UserDefaults.standard.set(try NSKeyedArchiver.archivedData(withRootObject: got.newAnchor, requiringSecureCoding: true), forKey: key)
            if records.count < limit { return }
        }
    }

    /// A sample as Archive's other health connectors keep one: its kind in their words, like steps or heart_rate; a workout goes to workouts,
    /// with its route; what has no number of its own goes to records, as FHIR.
    private func record(_ s: HKSample) async -> [String: Any] {
        if let r = await fhir(s) { return ["id": s.uuid.uuidString, "data": r] }
        var d: [String: Any] = ["start": s.startDate.iso, "end": s.endDate.iso, "source": s.sourceRevision.source.name]
        if let device = s.device?.model ?? s.device?.name { d["device"] = device }
        switch s {
        case let s as HKQuantitySample:
            d["kind"] = Self.kinds[s.quantityType.identifier] ?? Self.snake(s.quantityType.identifier)
            let unit = units[s.quantityType] ?? HKUnit.count()
            if s.quantity.is(compatibleWith: unit) {
                d["value"] = s.quantity.doubleValue(for: unit)
                d["unit"] = unit.unitString
            }
        case let s as HKCategorySample:
            if s.categoryType == HKCategoryType(.sleepAnalysis) {
                d["kind"] = "sleep_" + (["in_bed", "asleep", "awake", "core", "deep", "rem"][safe: s.value] ?? "asleep")
            } else {
                d["kind"] = Self.kinds[s.categoryType.identifier] ?? Self.snake(s.categoryType.identifier)
            }
            if Self.durations.contains(HKCategoryTypeIdentifier(rawValue: s.categoryType.identifier)) {
                d["value"] = s.endDate.timeIntervalSince(s.startDate) / 60
                d["unit"] = "min"
            } else {
                d["value"] = s.value
                d["unit"] = "level"
            }
        case let s as HKWorkout:
            d["sport"] = s.workoutActivityType.name
            d["seconds"] = s.duration
            if let kcal = s.statistics(for: HKQuantityType(.activeEnergyBurned))?.sumQuantity() {
                d["kcal"] = kcal.doubleValue(for: .kilocalorie())
            }
            if let m = s.statistics(for: HKQuantityType(.distanceWalkingRunning))?.sumQuantity() ?? s.statistics(for: HKQuantityType(.distanceCycling))?.sumQuantity() {
                d["meters"] = m.doubleValue(for: .meter())
            }
            if let gpx = await route(s) { d["route"] = Data(gpx.utf8).base64EncodedString() }
        default: break
        }
        return ["id": s.uuid.uuidString, "data": d]
    }

    /// HealthKit's names in the words Oura's, Fitbit's and Whoop's samples use; the rest in snake_case, like dietary_water.
    static let kinds: [String: String] = [
        HKQuantityTypeIdentifier.stepCount.rawValue: "steps", HKQuantityTypeIdentifier.heartRate.rawValue: "heart_rate",
        HKQuantityTypeIdentifier.restingHeartRate.rawValue: "resting_heart_rate", HKQuantityTypeIdentifier.heartRateVariabilitySDNN.rawValue: "hrv",
        HKQuantityTypeIdentifier.oxygenSaturation.rawValue: "spo2", HKQuantityTypeIdentifier.bodyMass.rawValue: "weight",
        HKQuantityTypeIdentifier.bodyFatPercentage.rawValue: "body_fat", HKQuantityTypeIdentifier.vo2Max.rawValue: "vo2_max",
        HKQuantityTypeIdentifier.activeEnergyBurned.rawValue: "active_calories", HKQuantityTypeIdentifier.basalEnergyBurned.rawValue: "resting_calories",
        HKQuantityTypeIdentifier.distanceWalkingRunning.rawValue: "distance", HKQuantityTypeIdentifier.appleExerciseTime.rawValue: "exercise_minutes",
        HKQuantityTypeIdentifier.appleStandTime.rawValue: "stand_minutes", HKQuantityTypeIdentifier.dietaryEnergyConsumed.rawValue: "calories_eaten",
        HKCategoryTypeIdentifier.mindfulSession.rawValue: "mindful",
    ]

    static func snake(_ id: String) -> String {
        let name = id.replacingOccurrences(of: #"^HK(Quantity|Category)TypeIdentifier"#, with: "", options: .regularExpression)
        return name.replacingOccurrences(of: #"([a-z0-9])([A-Z])"#, with: "$1_$2", options: .regularExpression).lowercased()
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

    /// Each photo and video itself, full size, after its record: oldest first, on Wi-Fi only, from iCloud where the phone keeps only a small copy.
    private func originals() async throws {
        guard PHPhotoLibrary.authorizationStatus(for: .readWrite) == .authorized || PHPhotoLibrary.authorizationStatus(for: .readWrite) == .limited,
              UserDefaults.standard.object(forKey: "photos.listed") == nil else { return }
        let doneFile = URL.applicationSupportDirectory.appending(path: "originals.json")
        var done = Set((try? JSONDecoder().decode([String].self, from: Data(contentsOf: doneFile))) ?? [])
        func save() throws {
            try FileManager.default.createDirectory(at: .applicationSupportDirectory, withIntermediateDirectories: true)
            try JSONEncoder().encode(Array(done)).write(to: doneFile, options: .completeFileProtectionUntilFirstUserAuthentication)
        }
        defer { try? save() }
        let opts = PHFetchOptions()
        opts.sortDescriptors = [NSSortDescriptor(key: "creationDate", ascending: true)]
        let assets = PHAsset.fetchAssets(with: opts)
        for i in 0 ..< assets.count {
            try Task.checkCancellation()
            let a = assets.object(at: i)
            guard !done.contains(a.localIdentifier),
                  let res = PHAssetResource.assetResources(for: a).first(where: { [.photo, .video, .audio].contains($0.type) }) else { continue }
            let tmp = FileManager.default.temporaryDirectory.appending(path: UUID().uuidString)
            defer { try? FileManager.default.removeItem(at: tmp) }
            let o = PHAssetResourceRequestOptions()
            o.isNetworkAccessAllowed = true
            try await PHAssetResourceManager.default().writeData(for: res, toFile: tmp, options: o)
            do {
                try await Client.shared.original(a.localIdentifier, name: res.originalFilename, file: tmp)
            } catch let ClientError.refused(status, said) where status == 404 && said.contains("no photo") {
                continue
            }
            done.insert(a.localIdentifier)
            await Status.shared.add("originals", 1)
            if done.count % 20 == 0 { try save() }
        }
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

    // MARK: Reminders

    /// Reminders, done or not, listed whole when they differ from the last listing, as events are.
    private func reminders() async throws {
        guard EKEventStore.authorizationStatus(for: .reminder) == .fullAccess else { return }
        let store = EKEventStore()
        let records: [[String: Any]] = await withCheckedContinuation { done in
            store.fetchReminders(matching: store.predicateForReminders(in: nil)) { found in
                done.resume(returning: (found ?? []).map(Self.reminder))
            }
        }
        let sum = Data(SHA256.hash(data: try JSONSerialization.data(withJSONObject: records, options: .sortedKeys))).base64URL
        if UserDefaults.standard.string(forKey: "reminders.sum") == sum { return }
        if records.isEmpty {
            try await pushed("reminders", [], full: true)
        }
        for start in stride(from: 0, to: records.count, by: 200) {
            let end = min(start + 200, records.count)
            try await pushed("reminders", Array(records[start ..< end]), full: true, more: end < records.count)
        }
        UserDefaults.standard.set(sum, forKey: "reminders.sum")
    }

    private static func reminder(_ r: EKReminder) -> [String: Any] {
        var d: [String: Any] = ["list": r.calendar.title, "title": r.title ?? "", "priority": r.priority, "done": r.isCompleted]
        if let n = r.notes { d["notes"] = n }
        if let u = r.url { d["url"] = u.absoluteString }
        if let t = r.dueDateComponents?.date { d["due"] = t.iso }
        if let t = r.startDateComponents?.date { d["start"] = t.iso }
        if let t = r.completionDate { d["completed"] = t.iso }
        if let t = r.creationDate { d["created"] = t.iso }
        if let t = r.lastModifiedDate { d["modified"] = t.iso }
        return ["id": r.calendarItemIdentifier, "data": d]
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
        case .running: "running"
        case .walking: "walking"
        case .cycling: "cycling"
        case .swimming: "swimming"
        case .hiking: "hiking"
        case .yoga: "yoga"
        case .functionalStrengthTraining: "functional strength training"
        case .traditionalStrengthTraining: "traditional strength training"
        case .highIntensityIntervalTraining: "high intensity interval training"
        case .rowing: "rowing"
        case .elliptical: "elliptical"
        case .dance: "dance"
        case .pilates: "pilates"
        case .tennis: "tennis"
        case .golf: "golf"
        case .soccer: "soccer"
        case .basketball: "basketball"
        default: "workout"
        }
    }
}
