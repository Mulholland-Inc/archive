import BackgroundTasks
import Contacts
import CoreLocation
import EventKit
import HealthKit
import Photos
import SwiftUI
import WidgetKit

@main
struct ArchiveApp: App {
    @UIApplicationDelegateAdaptor private var delegate: Delegate
    @Environment(\.scenePhase) private var phase

    var body: some Scene {
        WindowGroup {
            Root().environmentObject(Status.shared)
        }
        .onChange(of: phase) { _, now in
            if now == .active { Task { await Syncer.shared.all() } }
            if now == .background { Delegate.schedule() }
        }
    }
}

/// Wakes the app in the background: HealthKit's observers, a refresh every hour or so, and a longer run for the first listing of a big library.
final class Delegate: NSObject, UIApplicationDelegate {
    static let refresh = "dev.archive.app.refresh"
    static let backfill = "dev.archive.app.backfill"

    func application(_: UIApplication, didFinishLaunchingWithOptions _: [UIApplication.LaunchOptionsKey: Any]? = nil) -> Bool {
        for id in [Self.refresh, Self.backfill] {
            BGTaskScheduler.shared.register(forTaskWithIdentifier: id, using: nil) { task in
                let run = Task {
                    await Syncer.shared.all()
                    task.setTaskCompleted(success: true)
                }
                task.expirationHandler = { run.cancel() }
                Delegate.schedule()
            }
        }
        if Status.shared.onboarded { Watcher.watch() }
        _ = Places.shared
        return true
    }

    static func schedule() {
        let r = BGAppRefreshTaskRequest(identifier: refresh)
        r.earliestBeginDate = .now + 3600
        try? BGTaskScheduler.shared.submit(r)
        let p = BGProcessingTaskRequest(identifier: backfill)
        p.requiresNetworkConnectivity = true
        try? BGTaskScheduler.shared.submit(p)
    }
}

/// Syncs when something changes: Health wakes the app even in the background; Photos, Contacts and Calendar tell it while it runs.
/// Only once the user has answered, since watching Photos asks for it.
final class Watcher: NSObject, PHPhotoLibraryChangeObserver {
    static let shared = Watcher()

    static func watch() {
        Syncer.observe()
        PHPhotoLibrary.shared().register(shared)
        for name in [Notification.Name.CNContactStoreDidChange, .EKEventStoreChanged] {  // reminders change the event store too
            NotificationCenter.default.addObserver(forName: name, object: nil, queue: nil) { _ in Task { await Syncer.shared.all() } }
        }
    }

    func photoLibraryDidChange(_: PHChange) { Task { await Syncer.shared.all() } }
}

/// What the app shows: whether it is set up, and per stream how much it sent and when, kept across launches.
@MainActor final class Status: ObservableObject {
    static let shared = Status()
    static var streams: [(String, String, String)] {
        [("health", "Health", "heart"), ("photos", "Photos", "photo"), ("originals", "Originals", "photo.stack"), ("contacts", "Contacts", "person.crop.circle"),
         ("events", "Calendar", "calendar"), ("reminders", "Reminders", "checklist"), ("places", "Places", "mappin.and.ellipse")]
            + (Syncer.walletAvailable ? [("wallet", "Wallet", "wallet.pass")] : [])
    }

    /// The row each stream counts toward.
    nonisolated static let row = ["workouts": "health", "sleep": "health", "days": "health", "records": "health",
                                  "wallet_accounts": "wallet", "wallet_transactions": "wallet"]

    @Published var signedIn = Client.shared.signedIn
    @Published var onboarded = UserDefaults.standard.bool(forKey: "onboarded")
    @Published var sent: [String: Int] = Group.sent
    @Published var at: [String: Date] = Group.at
    @Published var errors: [String: String] = [:]

    func add(_ stream: String, _ n: Int) {
        sent[stream, default: 0] += n
        Group.defaults.set(sent, forKey: "sent")
    }

    func synced(_ stream: String, _ err: Error?) {
        if case .signedOut = err as? ClientError { signedIn = false }
        errors[stream] = err?.localizedDescription
        if err == nil {
            at[stream] = .now
            Group.defaults.set(at, forKey: "at")
            WidgetCenter.shared.reloadAllTimelines()
        }
    }

    /// Asks for everything at once: Health's sheets, then Photos', Contacts', Calendar's, Reminders', Location's and Wallet's.
    func allow() async {
        if HKHealthStore.isHealthDataAvailable() {
            // Health Records has sheets of its own, and closing them closes the request, so they come after the rest.
            try? await Syncer.health.requestAuthorization(toShare: Sample.writes, read: Syncer.reads.subtracting(Syncer.clinical))
            // A sheet asked for while the last one is still closing never shows, so each waits a moment.
            try? await Task.sleep(for: .seconds(1))
            if !Syncer.clinical.isEmpty { try? await Syncer.health.requestAuthorization(toShare: [], read: Set(Syncer.clinical)) }
            try? await Task.sleep(for: .seconds(1))
            if #available(iOS 26, *) {
                try? await Syncer.health.requestPerObjectReadAuthorization(for: HKObjectType.userAnnotatedMedicationType(), predicate: nil)
            }
        }
        _ = await PHPhotoLibrary.requestAuthorization(for: .readWrite)
        _ = try? await CNContactStore().requestAccess(for: .contacts)
        let events = EKEventStore()
        _ = try? await events.requestFullAccessToEvents()
        _ = try? await events.requestFullAccessToReminders()
        Places.shared.ask()
        await Syncer.shared.askWallet()
        onboarded = true
        UserDefaults.standard.set(true, forKey: "onboarded")
        Watcher.watch()
        await Sample.add()
        await Syncer.shared.all()
    }

    func signOut() async {
        await Client.shared.signOut()
        signedIn = false
    }
}

/// In the Simulator, which has no Health data or events of its own, -sample writes a few, real HealthKit samples and an event to sync.
enum Sample {
    static var on: Bool {
        #if targetEnvironment(simulator)
            ProcessInfo.processInfo.arguments.contains("-sample")
        #else
            false
        #endif
    }

    static var writes: Set<HKSampleType> {
        guard on else { return [] }
        var w: Set<HKSampleType> = [HKQuantityType(.stepCount), HKQuantityType(.heartRate), HKCategoryType(.sleepAnalysis), HKQuantityType(.activeEnergyBurned),
                                    HKQuantityType(.distanceWalkingRunning), HKCategoryType(.menstrualFlow), HKWorkoutType.workoutType(), HKSeriesType.workoutRoute()]
        if #available(iOS 18, *) { w.insert(HKObjectType.stateOfMindType()) }
        return w
    }

    static func add() async {
        guard on else { return }
        let now = Date.now
        var samples: [HKSample] = []
        for h in 1 ... 24 {
            let start = now - Double(h * 3600)
            let steps = HKQuantity(unit: .count(), doubleValue: Double(200 + h * 37 % 900))
            samples.append(HKQuantitySample(type: HKQuantityType(.stepCount), quantity: steps, start: start, end: start + 3600))
        }
        for i in 1 ... 12 {
            let at = now - Double(i * 1800)
            let bpm = HKQuantity(unit: .count().unitDivided(by: .minute()), doubleValue: Double(58 + i * 7 % 30))
            samples.append(HKQuantitySample(type: HKQuantityType(.heartRate), quantity: bpm, start: at, end: at))
        }
        let night = Calendar.current.startOfDay(for: now)
        let stages: [(HKCategoryValueSleepAnalysis, Double, Double)] = [(.asleepCore, -3, -1), (.asleepDeep, -1, 0.5), (.asleepREM, 0.5, 2), (.asleepCore, 2, 6.5)]
        for (stage, from, to) in stages {
            samples.append(HKCategorySample(type: HKCategoryType(.sleepAnalysis), value: stage.rawValue, start: night + from * 3600, end: night + to * 3600))
        }
        let flow = HKCategorySample(type: HKCategoryType(.menstrualFlow), value: HKCategoryValueMenstrualFlow.medium.rawValue, start: night - 86400 * 3,
                                    end: night - 86400 * 3, metadata: [HKMetadataKeyMenstrualCycleStart: true])
        samples.append(flow)
        if #available(iOS 18, *) {
            samples.append(HKStateOfMind(date: now - 3600, kind: .dailyMood, valence: 0.6, labels: [.happy, .grateful], associations: [.family]))
        }
        try? await Syncer.health.save(samples)
        await run(at: now - 86400)
        await Syncer.shared.visit(arrived: now - 7200, departed: now - 3600, at: .init(latitude: 34.0007, longitude: -118.4649), accuracy: 30)
        let store = EKEventStore()
        if let list = store.defaultCalendarForNewReminders() {
            let r = EKReminder(eventStore: store)
            r.calendar = list
            r.title = "Renew passport"
            r.dueDateComponents = Calendar.current.dateComponents([.year, .month, .day], from: now + 86400 * 14)
            try? store.save(r, commit: true)
        }
        if let cal = store.defaultCalendarForNewEvents {
            let e = EKEvent(eventStore: store)
            e.calendar = cal
            e.title = "Dinner with Kate"
            e.location = "Gjelina"
            e.startDate = Calendar.current.date(bySettingHour: 19, minute: 0, second: 0, of: now + 86400)!
            e.endDate = e.startDate + 7200
            try? store.save(e, span: .thisEvent)
        }
    }

    /// A 5 km run along the beach in Santa Monica, with its route.
    private static func run(at start: Date) async {
        let config = HKWorkoutConfiguration()
        config.activityType = .running
        config.locationType = .outdoor
        let b = HKWorkoutBuilder(healthStore: Syncer.health, configuration: config, device: nil)
        let end = start + 1800
        do {
            try await b.beginCollection(at: start)
            try await b.addSamples([HKQuantitySample(type: HKQuantityType(.distanceWalkingRunning), quantity: HKQuantity(unit: .meter(), doubleValue: 5000), start: start, end: end),
                                    HKQuantitySample(type: HKQuantityType(.activeEnergyBurned), quantity: HKQuantity(unit: .kilocalorie(), doubleValue: 380), start: start, end: end)])
            try await b.endCollection(at: end)
            guard let w = try await b.finishWorkout() else { return }
            let route = HKWorkoutRouteBuilder(healthStore: Syncer.health, device: nil)
            let points = (0 ... 60).map { i in
                CLLocation(coordinate: .init(latitude: 34.0100 - Double(i) * 0.0007, longitude: -118.4960 + Double(i) * 0.0004), altitude: 5, horizontalAccuracy: 5,
                           verticalAccuracy: 5, timestamp: start + Double(i) * 30)
            }
            try await route.insertRouteData(points)
            try await route.finishRoute(with: w, metadata: nil)
        } catch {}
    }
}
