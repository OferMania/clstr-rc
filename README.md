# clstr-rc

Fork of [clstr-io/clstr](https://github.com/clstr-io/clstr) (MIT licensed, original LICENSE preserved unchanged), modified for RamCourt usage.

To build, do:

```console
$ go build -o clstr ./cmd/clstr
```

## What got changed

For RamCourt, http_api tests expects JSON responses for all requests, and JSON body as input for all requests except GET. RamCourt outputs version as UUID string when creating, reading, or updating a key-value entry, and you must supply that field as input when making a subsequent update (PUT or DELETE) on that same key, or else the update is prohibited. When a key is deleted, the version is nil (all-zeroes UUID). Similarly, RamCourt also outputs table_version as UUID string in all API interactions of the global table, including GET on table/, and to clear that table, you need to supply table_version as input when calling DELETE on table/.

The test to perform 100 simultaneous writes on a single key has been modified to verify that 99 requests fail & exactly one succeeds. RamCourt by design requires different clients to be on the correct "assumptions" about a key, including its version, in order to be able to update the value.

Also, some behaviors differ deliberately from upstream clstr’s key-value tests. Deleting a key that doesn’t exist fails (409 if a version was supplied, 400 if not) instead of succeeding idempotently. Keys also expire (omitted duration_secs means a 300-second default, 0 means never). The http_api suite has tests for expiry, never-expiring keys, and the default TTL.

Please note that at the moment, only http_api tests have been modified for use on RamCourt, but not any of the other clstr tests. The longterm goal is to modify other tests in similar manner to work with RamCourt's unique quirks.

## RamCourt Known Limitations

- RamCourt does not yet support multiple tables. There is for now only one global table covering all keys and values. Once table-name is introduced, there will likely become a table/{table-name} endpoint, and interactions for that table are expected to go there.

- RamCourt does not yet support backup propagation between nodes in a cluster, or features corresponding to remaining tests, but that will be adjusted once that feature is added.

## Installing this fork's CLI

```console
$ go install github.com/OferMania/clstr-rc/cmd/clstr@latest
```

(Requires Go 1.26 or newer. Homebrew install isn't set up for this fork; build from source instead.)

# `clstr` CLI

_Learn distributed systems by building them from scratch._

Progressive challenges to learn distributed systems and other complex systems by implementing them yourself.

## How it Works

Write code, run tests, get detailed feedback. Progress through stages as you build real systems.

Learn more at [clstr.io](https://clstr.io).
