import CoreLocation
import Foundation

/// The places the phone saw the user stay a while, from iOS's visits, which wake the app even when it isn't running, at little cost to the battery.
/// Each waits in a queue until Archive has it, with the place's name and address from Apple's maps.
final class Places: NSObject, CLLocationManagerDelegate {
    static let shared = Places()
    private let manager = CLLocationManager()

    override init() {
        super.init()
        manager.delegate = self
    }

    func ask() {
        manager.requestAlwaysAuthorization()
    }

    func locationManagerDidChangeAuthorization(_ m: CLLocationManager) {
        switch m.authorizationStatus {
        case .authorizedAlways:
            m.startMonitoringVisits()
        case .authorizedWhenInUse:
            m.startMonitoringVisits()
            m.requestAlwaysAuthorization()
        default:
            m.stopMonitoringVisits()
        }
    }

    func locationManager(_: CLLocationManager, didVisit v: CLVisit) {
        let arrived = v.arrivalDate == .distantPast ? nil : v.arrivalDate, departed = v.departureDate == .distantFuture ? nil : v.departureDate
        Task { await Syncer.shared.visit(arrived: arrived, departed: departed, at: v.coordinate, accuracy: v.horizontalAccuracy) }
    }
}

extension Syncer {
    private static let queued = "places.queue"

    func visit(arrived: Date?, departed: Date?, at c: CLLocationCoordinate2D, accuracy: Double) async {
        guard let when = arrived ?? departed else { return }
        var d: [String: Any] = ["latitude": c.latitude, "longitude": c.longitude, "accuracy": accuracy]
        if let arrived { d["arrived"] = arrived.iso }
        if let departed { d["departed"] = departed.iso }
        if let p = try? await CLGeocoder().reverseGeocodeLocation(CLLocation(latitude: c.latitude, longitude: c.longitude)).first {
            d["title"] = p.areasOfInterest?.first ?? p.name
            d["address"] = [p.subThoroughfare, p.thoroughfare].compactMap { $0 }.joined(separator: " ")
            d["city"] = p.locality
            d["country"] = p.country
        }
        // The arrival names the visit, so the departure iOS reports later lands on the same one.
        let id = "\(Int(when.timeIntervalSince1970))/\(String(format: "%.4f,%.4f", c.latitude, c.longitude))"
        var q = UserDefaults.standard.array(forKey: Self.queued) as? [[String: Any]] ?? []
        q.append(["id": id, "data": d])
        UserDefaults.standard.set(q, forKey: Self.queued)
        await all()
    }

    func places() async throws {
        let q = UserDefaults.standard.array(forKey: Self.queued) as? [[String: Any]] ?? []
        guard !q.isEmpty else { return }
        try await pushed("places", q)
        let now = UserDefaults.standard.array(forKey: Self.queued) as? [[String: Any]] ?? []
        UserDefaults.standard.set(Array(now.dropFirst(q.count)), forKey: Self.queued)
    }
}
