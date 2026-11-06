import CoreLocation
import CryptoKit
import Foundation
import HealthKit

/// What Health has beyond plain samples: its records as FHIR, workouts' routes, and the nights, days and medications made of what it keeps.
extension Syncer {
    // MARK: Records

    /// A sample with no number of its own as the FHIR resource Archive's health_record keeps, the same as MyChart's: Health Records as the
    /// hospital sent them, the rest made into one. Nil for a plain sample.
    func fhir(_ s: HKSample) async -> [String: Any]? {
        let at = s.startDate.iso
        switch s {
        case let r as HKClinicalRecord:
            guard let data = r.fhirResource?.data, let resource = try? JSONSerialization.jsonObject(with: data) else { return nil }
            return ["resource": resource, "organization": r.sourceRevision.source.name]
        case let e as HKElectrocardiogram:
            var o = Self.observation("Electrocardiogram", loinc: "11524-6", category: "procedure", at: at)
            o["valueString"] = e.classification.name
            if let bpm = e.averageHeartRate?.doubleValue(for: .count().unitDivided(by: .minute())) {
                o["component"] = [["code": ["text": "Average heart rate"], "valueQuantity": ["value": bpm, "unit": "/min"]]]
            }
            if e.symptomsStatus == .present { o["note"] = [["text": "Symptoms logged"]] }
            return ["resource": o, "organization": s.sourceRevision.source.name]
        case let a as HKAudiogramSample:
            var o = Self.observation("Hearing test", loinc: "28615-3", category: "exam", at: at)
            o["component"] = a.sensitivityPoints.flatMap(Self.hearing)
            return ["resource": o, "organization": s.sourceRevision.source.name]
        default:
            break
        }
        if #available(iOS 18, *), let m = s as? HKStateOfMind {
            var o = Self.observation(m.kind == .dailyMood ? "Mood" : "Emotion", loinc: nil, category: "survey", at: at)
            o["valueQuantity"] = ["value": m.valence, "unit": "valence, -1 to 1"]
            let labels = m.labels.compactMap { Self.feelings[safe: $0.rawValue - 1] }, about = m.associations.compactMap { Self.about[safe: $0.rawValue - 1] }
            o["valueString"] = ([Self.valence[safe: m.valenceClassification.rawValue - 1] ?? "neutral"] + labels).joined(separator: ", ")
            if !about.isEmpty { o["note"] = [["text": "About: " + about.joined(separator: ", ")]] }
            return ["resource": o, "organization": s.sourceRevision.source.name]
        }
        if #available(iOS 26, *), let d = s as? HKMedicationDoseEvent {
            var name = "Medication"
            if let m = try? await medications().first(where: { $0.medication.identifier == d.medicationConceptIdentifier }) {
                name = m.nickname ?? m.medication.displayText
            }
            var o: [String: Any] = ["resourceType": "MedicationAdministration", "effectiveDateTime": at,
                                    "status": [.taken: "completed", .skipped: "not-done"][d.logStatus] ?? "unknown",
                                    "medicationCodeableConcept": ["text": name]]
            if let dose = d.doseQuantity { o["dosage"] = ["dose": ["value": dose, "unit": d.unit.unitString]] }
            if d.scheduleType == .asNeeded { o["category"] = ["text": "as needed"] }
            return ["resource": o, "organization": s.sourceRevision.source.name]
        }
        return nil
    }

    // State of Mind's words, by their raw values, which the SDK numbers from 1.
    static let valence = ["very unpleasant", "unpleasant", "slightly unpleasant", "neutral", "slightly pleasant", "pleasant", "very pleasant"]
    static let feelings = ["amazed", "amused", "angry", "anxious", "ashamed", "brave", "calm", "content", "disappointed", "discouraged", "disgusted",
                           "embarrassed", "excited", "frustrated", "grateful", "guilty", "happy", "hopeless", "irritated", "jealous", "joyful", "lonely",
                           "passionate", "peaceful", "proud", "relieved", "sad", "scared", "stressed", "surprised", "worried", "annoyed", "confident",
                           "drained", "hopeful", "indifferent", "overwhelmed", "satisfied"]
    static let about = ["community", "current events", "dating", "education", "family", "fitness", "friends", "health", "hobbies", "identity", "money",
                        "partner", "self care", "spirituality", "tasks", "travel", "work", "weather"]

    static func observation(_ text: String, loinc: String?, category: String, at: String) -> [String: Any] {
        var code: [String: Any] = ["text": text]
        if let loinc { code["coding"] = [["system": "http://loinc.org", "code": loinc, "display": text]] }
        return ["resourceType": "Observation", "status": "final", "code": code, "effectiveDateTime": at,
                "category": [["coding": [["system": "http://terminology.hl7.org/CodeSystem/observation-category", "code": category]]]]]
    }

    private static func hearing(_ p: HKAudiogramSensitivityPoint) -> [[String: Any]] {
        let hz = p.frequency.doubleValue(for: .hertz())
        var ears: [(String, Double)] = []
        if #available(iOS 18.1, *) {
            ears = p.tests.map { ($0.side == .left ? "Left" : "Right", $0.sensitivity.doubleValue(for: .decibelHearingLevel())) }
        } else {
            if let l = p.leftEarSensitivity { ears.append(("Left", l.doubleValue(for: .decibelHearingLevel()))) }
            if let r = p.rightEarSensitivity { ears.append(("Right", r.doubleValue(for: .decibelHearingLevel()))) }
        }
        return ears.map { ["code": ["text": "\($0.0) ear, \(Int(hz)) Hz"], "valueQuantity": ["value": $0.1, "unit": "dB HL"]] }
    }

    // MARK: Routes

    /// A workout's route as GPX, the format runners' apps trade; nil without one.
    func route(_ w: HKWorkout) async -> String? {
        let q = HKSampleQueryDescriptor(predicates: [.workoutRoute(HKQuery.predicateForObjects(from: w))], sortDescriptors: [])
        guard let routes = try? await q.result(for: Self.health), !routes.isEmpty else { return nil }
        var points: [CLLocation] = []
        for r in routes {
            let all = HKWorkoutRouteQueryDescriptor(r)
            do {
                for try await l in all.results(for: Self.health) { points.append(l) }
            } catch {
                return nil
            }
        }
        guard !points.isEmpty else { return nil }
        let pts = points.map {
            "<trkpt lat=\"\($0.coordinate.latitude)\" lon=\"\($0.coordinate.longitude)\"><ele>\($0.altitude)</ele><time>\($0.timestamp.iso)</time></trkpt>"
        }
        return "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<gpx version=\"1.1\" creator=\"Archive\" xmlns=\"http://www.topografix.com/GPX/1/1\">" +
            "<trk><name>\(w.workoutActivityType.name)</name><trkseg>" + pts.joined() + "</trkseg></trk></gpx>\n"
    }

    // MARK: Made of samples

    /// Nights, days and medications, each from where the last run left them.
    func healthMade() async throws {
        guard Client.shared.signedIn, HKHealthStore.isHealthDataAvailable() else { return }
        try await nights()
        try await days()
        try await medicationList()
    }

    private var start: Date {
        max(Self.health.earliestPermittedSampleDate(), Calendar.current.date(byAdding: .year, value: -10, to: .now)!)
    }

    /// Each night or nap, the sleep stages one source has between going to bed and getting up, as Oura and WHOOP keep a night. Where a
    /// watch and a phone both tracked it, the one that heard more sleep. From two days before the last night sent, as late stages come in.
    private func nights() async throws {
        let from = (UserDefaults.standard.object(forKey: "nights.from") as? Date) ?? start
        let q = HKSampleQueryDescriptor(predicates: [.categorySample(type: HKCategoryType(.sleepAnalysis), predicate: HKQuery.predicateForSamples(withStart: from, end: nil))],
                                        sortDescriptors: [SortDescriptor(\.startDate)])
        let samples = try await q.result(for: Self.health)
        var groups: [[HKCategorySample]] = []
        for s in samples {
            if let last = groups.last?.map(\.endDate).max(), s.startDate.timeIntervalSince(last) < 2 * 3600 {
                groups[groups.count - 1].append(s)
            } else {
                groups.append([s])
            }
        }
        var records: [[String: Any]] = []
        for g in groups {
            let asleep = { (s: HKCategorySample) in HKCategoryValueSleepAnalysis.allAsleepValues.map(\.rawValue).contains(s.value) }
            let by = Dictionary(grouping: g) { $0.sourceRevision.source.bundleIdentifier }
            guard let best = by.values.max(by: { a, b in a.filter(asleep).duration < b.filter(asleep).duration }), let first = best.first else { continue }
            let span = (best.map(\.startDate).min()!, best.map(\.endDate).max()!)
            let stage = { (v: HKCategoryValueSleepAnalysis) in best.filter { $0.value == v.rawValue }.duration / 60 }
            let slept = best.filter(asleep).duration / 60
            let bed = best.contains { $0.value == HKCategoryValueSleepAnalysis.inBed.rawValue } ? stage(.inBed) : span.1.timeIntervalSince(span.0) / 60
            let hour = Calendar.current.component(.hour, from: span.0)
            var d: [String: Any] = ["day": Self.day(span.1), "start": span.0.iso, "end": span.1.iso, "asleep": slept, "deep": stage(.asleepDeep),
                                    "rem": stage(.asleepREM), "core": stage(.asleepCore) + stage(.asleepUnspecified), "awake": stage(.awake), "inBed": bed,
                                    "nap": slept < 180 && hour >= 9 && hour < 19, "source": first.sourceRevision.source.name]
            if let device = first.device?.model { d["device"] = device }
            let during = HKQuery.predicateForSamples(withStart: span.0, end: span.1)
            for (key, id, opt, unit) in [("heartRate", HKQuantityTypeIdentifier.heartRate, HKStatisticsOptions.discreteAverage, HKUnit.count().unitDivided(by: .minute())),
                                         ("lowestHeartRate", .heartRate, .discreteMin, .count().unitDivided(by: .minute())),
                                         ("hrv", .heartRateVariabilitySDNN, .discreteAverage, .secondUnit(with: .milli)),
                                         ("respiratoryRate", .respiratoryRate, .discreteAverage, .count().unitDivided(by: .minute()))] {
                let stat = HKStatisticsQueryDescriptor(predicate: .quantitySample(type: HKQuantityType(id), predicate: during), options: opt)
                guard let r = try? await stat.result(for: Self.health) else { continue }
                if let v = (opt == .discreteMin ? r.minimumQuantity() : r.averageQuantity())?.doubleValue(for: unit) { d[key] = v }
            }
            records.append(["id": first.uuid.uuidString, "data": d])
        }
        for start in stride(from: 0, to: records.count, by: 200) {
            try await pushed("sleep", Array(records[start ..< min(start + 200, records.count)]))
        }
        if let last = groups.last?.first?.startDate {
            UserDefaults.standard.set(Calendar.current.date(byAdding: .day, value: -2, to: last), forKey: "nights.from")
        }
    }

    /// A day's activity as Health's rings show it, the last three days again each time, as they fill in.
    private func days() async throws {
        let cal = Calendar.current
        let from = cal.startOfDay(for: (UserDefaults.standard.object(forKey: "days.from") as? Date) ?? start)
        let to = Date.now
        var days: [String: [String: Any]] = [:]
        let sums: [(String, HKQuantityTypeIdentifier, HKUnit)] = [("steps", .stepCount, .count()), ("activeCalories", .activeEnergyBurned, .kilocalorie()),
                                                                  ("restingCalories", .basalEnergyBurned, .kilocalorie()), ("meters", .distanceWalkingRunning, .meter()),
                                                                  ("exerciseMinutes", .appleExerciseTime, .minute()), ("flights", .flightsClimbed, .count())]
        let means: [(String, HKQuantityTypeIdentifier, HKUnit)] = [("restingHeartRate", .restingHeartRate, .count().unitDivided(by: .minute())),
                                                                   ("hrv", .heartRateVariabilitySDNN, .secondUnit(with: .milli))]
        for (key, id, unit, sum) in sums.map({ ($0, $1, $2, true) }) + means.map({ ($0, $1, $2, false) }) {
            let q = HKStatisticsCollectionQueryDescriptor(predicate: .quantitySample(type: HKQuantityType(id), predicate: HKQuery.predicateForSamples(withStart: from, end: to)),
                                                          options: sum ? .cumulativeSum : .discreteAverage, anchorDate: from, intervalComponents: DateComponents(day: 1))
            guard let c = try? await q.result(for: Self.health) else { continue }
            c.enumerateStatistics(from: from, to: to) { s, _ in
                if let v = (sum ? s.sumQuantity() : s.averageQuantity())?.doubleValue(for: unit) { days[Self.day(s.startDate), default: [:]][key] = v }
            }
        }
        let rings = HKActivitySummaryQueryDescriptor(predicate: HKQuery.predicate(forActivitySummariesBetweenStart: cal.dateComponents([.era, .year, .month, .day, .calendar], from: from),
                                                                                    end: cal.dateComponents([.era, .year, .month, .day, .calendar], from: to)))
        for a in (try? await rings.result(for: Self.health)) ?? [] {
            guard let date = a.dateComponents(for: cal).date else { continue }
            var d = days[Self.day(date), default: [:]]
            d["standHours"] = a.appleStandHours.doubleValue(for: .count())
            d["moveGoal"] = a.activeEnergyBurnedGoal.doubleValue(for: .kilocalorie())
            d["exerciseGoal"] = a.appleExerciseTimeGoal.doubleValue(for: .minute())
            d["standGoal"] = a.appleStandHoursGoal.doubleValue(for: .count())
            days[Self.day(date)] = d
        }
        var records: [[String: Any]] = []
        for (day, var d) in days.sorted(by: { $0.key < $1.key }) {
            if let a = d["activeCalories"] as? Double { d["calories"] = a + (d["restingCalories"] as? Double ?? 0) }
            d["day"] = day
            records.append(["id": day, "data": d])
        }
        for start in stride(from: 0, to: records.count, by: 500) {
            try await pushed("days", Array(records[start ..< min(start + 500, records.count)]))
        }
        UserDefaults.standard.set(cal.date(byAdding: .day, value: -2, to: cal.startOfDay(for: to)), forKey: "days.from")
    }

    // MARK: Medications

    @available(iOS 26, *)
    func medications() async throws -> [HKUserAnnotatedMedication] {
        try await HKUserAnnotatedMedicationQueryDescriptor().result(for: Self.health)
    }

    /// The medications the user chose to share from Health, as FHIR MedicationStatements, listed whole when they change.
    private func medicationList() async throws {
        guard #available(iOS 26, *) else { return }
        let records = try await medications().map { m -> [String: Any] in
            var concept: [String: Any] = ["text": m.medication.displayText]
            concept["coding"] = m.medication.relatedCodings.map { ["system": $0.system, "code": $0.code] }
            var r: [String: Any] = ["resourceType": "MedicationStatement", "status": m.isArchived ? "stopped" : "active", "medicationCodeableConcept": concept,
                                    "dosage": [["text": m.hasSchedule ? "On a schedule" : "As needed"]]]
            if let nick = m.nickname { r["note"] = [["text": nick]] }
            r["form"] = m.medication.generalForm.rawValue
            let id = (try? NSKeyedArchiver.archivedData(withRootObject: m.medication.identifier, requiringSecureCoding: true)).map { Data(SHA256.hash(data: $0)).base64URL }
            return ["id": "medication/" + (id ?? m.medication.displayText), "data": ["resource": r, "organization": "Apple Health"]]
        }
        let sum = Data(SHA256.hash(data: try JSONSerialization.data(withJSONObject: records, options: .sortedKeys))).base64URL
        if records.isEmpty || UserDefaults.standard.string(forKey: "medications.sum") == sum { return }
        try await pushed("records", records)
        UserDefaults.standard.set(sum, forKey: "medications.sum")
    }

    static func day(_ d: Date) -> String {
        let f = DateFormatter()
        f.calendar = Calendar(identifier: .gregorian)
        f.dateFormat = "yyyy-MM-dd"
        return f.string(from: d)
    }
}

extension [HKCategorySample] {
    /// The time the samples cover, counting time two of them share once.
    var duration: TimeInterval {
        var total: TimeInterval = 0, end = Date.distantPast
        for s in sorted(by: { $0.startDate < $1.startDate }) {
            let from = Swift.max(s.startDate, end)
            if s.endDate > from { total += s.endDate.timeIntervalSince(from) }
            end = Swift.max(end, s.endDate)
        }
        return total
    }
}

extension HKElectrocardiogram.Classification {
    var name: String {
        switch self {
        case .sinusRhythm: "Sinus rhythm"
        case .atrialFibrillation: "Atrial fibrillation"
        case .inconclusiveLowHeartRate: "Inconclusive: low heart rate"
        case .inconclusiveHighHeartRate: "Inconclusive: high heart rate"
        case .inconclusivePoorReading: "Inconclusive: poor reading"
        case .inconclusiveOther: "Inconclusive"
        default: "Not set"
        }
    }
}
