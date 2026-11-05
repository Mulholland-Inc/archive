import SwiftUI

struct Root: View {
    @EnvironmentObject var status: Status

    var body: some View {
        if !status.signedIn {
            SignIn()
        } else if !status.onboarded {
            Allow()
        } else {
            Synced()
        }
    }
}

struct SignIn: View {
    @EnvironmentObject var status: Status
    @State private var busy = false
    @State private var error: String?

    var body: some View {
        VStack(spacing: 16) {
            Spacer()
            Image("Mark").resizable().scaledToFit().frame(width: 88).clipShape(RoundedRectangle(cornerRadius: 20))
            Text("Archive").font(.largeTitle.weight(.semibold))
            Spacer()
            if let error { Text(error).font(.footnote).foregroundStyle(.red).multilineTextAlignment(.center) }
            Button {
                busy = true
                Task {
                    do {
                        try await Client.shared.signIn()
                        status.signedIn = true
                        error = nil
                    } catch {
                        self.error = error.localizedDescription
                    }
                    busy = false
                }
            } label: {
                Text("Sign in").frame(maxWidth: .infinity)
            }
            .buttonStyle(.borderedProminent).controlSize(.large).disabled(busy)
        }
        .padding(24)
    }
}

struct Allow: View {
    @EnvironmentObject var status: Status
    @State private var busy = false

    var body: some View {
        VStack(alignment: .leading, spacing: 28) {
            Text("Keep your iPhone in Archive").font(.title.weight(.semibold)).padding(.top, 48)
            ForEach([("heart", "Health", "Activity, heart, sleep, body and workouts. Only you see it."),
                     ("photo", "Photos", "When and where each was taken, and a small copy."),
                     ("person.crop.circle", "Contacts", "Names, emails, phones and companies."),
                     ("calendar", "Calendar", "Events from a year back to two ahead.")], id: \.1) { icon, title, line in
                HStack(alignment: .top, spacing: 14) {
                    Image(systemName: icon).font(.title2).frame(width: 32)
                    VStack(alignment: .leading, spacing: 2) {
                        Text(title).font(.headline)
                        Text(line).font(.subheadline).foregroundStyle(.secondary)
                    }
                }
            }
            Spacer()
            Button {
                busy = true
                Task { await status.allow() }
            } label: {
                Text("Allow").frame(maxWidth: .infinity)
            }
            .buttonStyle(.borderedProminent).controlSize(.large).disabled(busy)
        }
        .padding(24)
    }
}

struct Synced: View {
    @EnvironmentObject var status: Status

    var body: some View {
        NavigationStack {
            List {
                ForEach(Status.streams, id: \.0) { id, title, icon in
                    HStack(spacing: 14) {
                        Image(systemName: icon).frame(width: 28)
                        VStack(alignment: .leading, spacing: 2) {
                            Text(title)
                            if let e = status.errors[id] {
                                Text(e).font(.caption).foregroundStyle(.red).lineLimit(2)
                            } else if let at = status.at[id] {
                                Text(at, format: .relative(presentation: .named)).font(.caption).foregroundStyle(.secondary)
                            }
                        }
                        Spacer()
                        Text((status.sent[id] ?? 0).formatted()).monospacedDigit().foregroundStyle(.secondary)
                    }
                }
            }
            .navigationTitle("Archive")
            .refreshable { await Syncer.shared.all() }
            .toolbar {
                Menu {
                    Button("Sign out", role: .destructive) { Task { await status.signOut() } }
                } label: {
                    Image(systemName: "ellipsis.circle")
                }
            }
        }
    }
}
