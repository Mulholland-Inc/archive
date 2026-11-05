import BackgroundTasks
import Contacts
import EventKit
import HealthKit
import Photos
import SwiftUI

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
        if Status.shared.onboarded { Syncer.observe() }
        for name in [Notification.Name.CNContactStoreDidChange, .EKEventStoreChanged] {
            NotificationCenter.default.addObserver(forName: name, object: nil, queue: nil) { _ in Task { await Syncer.shared.all() } }
        }
        PHPhotoLibrary.shared().register(Watcher.shared)
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

/// Photos tells a registered observer, not NotificationCenter, when the library changes while the app runs.
final class Watcher: NSObject, PHPhotoLibraryChangeObserver {
    static let shared = Watcher()
    func photoLibraryDidChange(_: PHChange) { Task { await Syncer.shared.all() } }
}

/// What the app shows: whether it is set up, and per stream how much it sent and when, kept across launches.
@MainActor final class Status: ObservableObject {
    static let shared = Status()
    static let streams = [("health", "Health", "heart"), ("photos", "Photos", "photo"), ("contacts", "Contacts", "person.crop.circle"), ("events", "Calendar", "calendar")]

    @Published var signedIn = Client.shared.signedIn
    @Published var onboarded = UserDefaults.standard.bool(forKey: "onboarded")
    @Published var sent: [String: Int] = UserDefaults.standard.dictionary(forKey: "sent") as? [String: Int] ?? [:]
    @Published var at: [String: Date] = UserDefaults.standard.dictionary(forKey: "at") as? [String: Date] ?? [:]
    @Published var errors: [String: String] = [:]

    func add(_ stream: String, _ n: Int) {
        sent[stream, default: 0] += n
        UserDefaults.standard.set(sent, forKey: "sent")
    }

    func synced(_ stream: String, _ err: Error?) {
        if case .signedOut = err as? ClientError { signedIn = false }
        errors[stream] = err?.localizedDescription
        if err == nil {
            at[stream] = .now
            UserDefaults.standard.set(at, forKey: "at")
        }
    }

    /// Asks for all four at once: Health's sheet, then Photos', Contacts' and Calendar's alerts.
    func allow() async {
        if HKHealthStore.isHealthDataAvailable() {
            try? await Syncer.health.requestAuthorization(toShare: Sample.writes, read: Set(Syncer.types))
        }
        _ = await PHPhotoLibrary.requestAuthorization(for: .readWrite)
        _ = try? await CNContactStore().requestAccess(for: .contacts)
        _ = try? await EKEventStore().requestFullAccessToEvents()
        onboarded = true
        UserDefaults.standard.set(true, forKey: "onboarded")
        Syncer.observe()
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
        on ? [HKQuantityType(.stepCount), HKQuantityType(.heartRate), HKCategoryType(.sleepAnalysis)] : []
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
        try? await Syncer.health.save(samples)
        let store = EKEventStore()
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
}
