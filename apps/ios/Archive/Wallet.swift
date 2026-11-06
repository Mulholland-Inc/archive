import FinanceKit
import Foundation

/// Apple Card, Apple Cash and Apple Savings from Wallet, through FinanceKit. It needs Apple's FinanceKit entitlement, which Apple grants on request,
/// and FinanceKit stops an app without it, so the build turns Wallet on (ARCHIVE_WALLET) only once the entitlement is in.
extension Syncer {
    static var walletAvailable: Bool {
        guard #available(iOS 17.4, *), Bundle.main.object(forInfoDictionaryKey: "ArchiveWallet") as? String == "YES" else { return false }
        return FinanceStore.isDataAvailable(.financialData)
    }

    func wallet() async throws {
        guard #available(iOS 17.4, *), Self.walletAvailable else { return }
        let store = FinanceStore.shared
        guard (try? await store.authorizationStatus()) == .authorized else { return }
        let accounts = try await store.accounts(query: AccountQuery())
        let balances = Dictionary(try await store.accountBalances(query: AccountBalanceQuery()).map { ($0.accountID, $0) }) { a, _ in a }
        let records = accounts.map { a -> [String: Any] in
            var d: [String: Any] = ["name": a.displayName, "institution": a.institutionName, "currency": a.currencyCode]
            d["kind"] = a.liabilityAccount == nil ? "depository" : "credit"
            if let b = balances[a.id] {
                if let v = b.booked { d["balance"] = v.signed }
                if let v = b.available { d["available"] = v.signed }
            }
            if let l = a.liabilityAccount?.creditInformation.creditLimit { d["limit"] = "\(l.amount)" }
            return ["id": a.id.uuidString, "data": d]
        }
        try await pushed("wallet_accounts", records, full: true)
        for a in accounts {
            let key = "wallet.token." + a.id.uuidString
            let token = UserDefaults.standard.data(forKey: key).flatMap { try? JSONDecoder().decode(FinanceStore.HistoryToken.self, from: $0) }
            for try await changes in store.transactionHistory(forAccountID: a.id, since: token, isMonitoring: false) {
                let records = (changes.inserted + changes.updated).map(Self.transaction) + changes.deleted.map { ["id": $0.uuidString, "deleted": true] }
                for start in stride(from: 0, to: records.count, by: 500) {
                    try await pushed("wallet_transactions", Array(records[start ..< min(start + 500, records.count)]))
                }
                UserDefaults.standard.set(try JSONEncoder().encode(changes.newToken), forKey: key)
            }
        }
    }

    @available(iOS 17.4, *)
    private static func transaction(_ t: FinanceKit.Transaction) -> [String: Any] {
        if t.status == .rejected { return ["id": t.id.uuidString, "deleted": true] }
        var d: [String: Any] = ["account": t.accountID.uuidString, "authorized": t.transactionDate.iso, "description": t.transactionDescription,
                                "amount": "\(t.creditDebitIndicator == .debit ? -t.transactionAmount.amount : t.transactionAmount.amount)",
                                "currency": t.transactionAmount.currencyCode, "kind": String(describing: t.transactionType),
                                "pending": t.status == .pending || t.status == .authorized]
        if let p = t.postedDate { d["posted"] = p.iso }
        if let m = t.merchantName { d["merchant"] = m }
        if let c = t.merchantCategoryCode { d["category"] = c.description }
        return ["id": t.id.uuidString, "data": d]
    }
}

@available(iOS 17.4, *)
extension Balance {
    /// What the account holds, as a decimal string: negative when it is owed.
    var signed: String { "\(creditDebitIndicator == .debit ? -amount.amount : amount.amount)" }
}

extension Syncer {
    func askWallet() async {
        guard #available(iOS 17.4, *), Self.walletAvailable else { return }
        _ = try? await FinanceStore.shared.requestAuthorization()
    }
}
