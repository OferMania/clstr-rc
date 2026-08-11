# clstr-rc

Fork of [clstr-io/clstr](https://github.com/clstr-io/clstr) (MIT licensed, original LICENSE preserved unchanged), modified for RamCourt usage.

To build, do:

```console
$ go build -o clstr ./cmd/clstr
```

## What got changed

For RamCourt, http_api tests expects JSON responses for all requests, and JSON body as input for all requests except GET. RamCourt outputs version as UUID string when creating, reading, or updating a key-value entry, and you must supply that field as input when making a subsequent update (PUT or DELETE) on that same key, or else the update is prohibited. When a key is deleted, the version is nil (all-zeroes UUID). Similarly, RamCourt also outputs table_version as UUID string in all API interactions of the global table, including GET on table/, and to clear that table, you need to supply table_version as input when calling DELETE on table/.

The test to perform 100 simultaneous writes on a single key has been modified to verify that 99 requests fail & exactly one succeeds. RamCourt by design requires different clients to be on the correct "assumptions" about a key, including its version, in order to be able to update the value.

Please note that at the moment, only http_api tests have been modified for use on RamCourt, but not any of the other clstr tests. The longterm goal is to modify other tests in similar manner to work with RamCourt's unique quirks.

## RamCourt Known Limitations

- RamCourt supports duration_secs as input and expiration_secs on the key-value entry itself, but does not yet clean the entry when the expiration point of time is reached. When it does, you can expect tests related to duration and expiration.

- RamCourt does not yet support multiple tables. There is for now only one global table covering all keys and values. Once table-name is introduced, there will likely become a table/{table-name} endpoint, and interactions for that table are expected to go there.

- RamCourt does not yet support backup propagation between nodes in a cluster, or features corresponding to remaining tests, but that will be adjusted once that feature is added.

## Installing this fork's CLI

```console
$ go install github.com/OferMania/clstr-rc/cmd/clstr@latest
```

(Requires GOPRIVATE and git auth configured for this repo — see repo notes. Homebrew install isn't set up for this fork; build from source instead.)

# `clstr` CLI

_Learn distributed systems by building them from scratch._

Progressive challenges to learn distributed systems and other complex systems by implementing them yourself.

## How it Works

Write code, run tests, get detailed feedback. Progress through stages as you build real systems.

Learn more at [clstr.io](https://clstr.io).
