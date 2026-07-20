# Readme

# V2 Endpoint
With the V2 REST API we started using a design driven process.

The API is defined using [OpenAPI v2](https://swagger.io/specification/v2/) in **algod.oas2.json**.

## Updating the V2 REST API

1. Document your changes by editing **algod.oas2.json**
2. Regenerate the endpoints by running **make generate**.
3. Update the implementation in **server/v2/handlers.go**. It is sometimes useful to consult **generated/\*/\*/routes.go** to make sure the handler properly implements **ServerInterface**.

### Adding a new V2 API
When adding a new endpoint to the V2 APIs, you will need to add `tags` to the path. The tags are a way of separating our
APIs into groups--the motivation of which is to more easily be able to conditionally enable and/or disable groups of
endpoints based on the use case for the node.

Each API in `algod.oas2.json`, except for some pre-existing `common` APIs, should have two tags.
1. Either `public` or `private`. This controls the type of authentication used by the API--the `public` APIs use the
`algod.token` token, while the `private` APIs use the admin token, found in `algod.admin.token` within the algod data
directory.
2. The type, or group, of API. This is currently `participating`, `nonparticipating`, `data`, or `experimental`, but
may expand in the future to encompass different sets of APIs. Additional APIs should be added to one of the existing
sets of tags based on its use case--unless you intend to create a new group in which case you will need to additionally
ensure your new APIs are registered.

For backwards compatibility, the default set of APIs registered will always be `participating` and `nonparticipating`
APIs.

The current set of API groups and some rough descriptions of how to think about them:
* `participating`
  * APIs used in forming blocks/transactions and generally advancing the chain. Things which use the txn pool,
participation keys, the agreement service, etc.
* `nonparticipating`
  * Generally available APIs used to do things such as fetch data. For example, GetGenesis, GetBlock, Catchpoint Catchup, etc.
* `data`
  * A special set of APIs which require manipulating the node state in order to provide additional data about the node state
at some predefined granularity. For example, SetSyncRound and GetLedgerStateDelta used together control and expose StateDelta objects
containing per-round ledger differences that get compacted when actually written to the ledger DB.
* `experimental`
  * APIs which are still in development and not ready to be generally released.

## What codegen tool is used?

We found that [oapi-codegen](https://github.com/deepmap/oapi-codegen)
produced the cleanest code, and had an easy to work with codebase. We
initially forked it in `algorand/oapi-codegen` but found that features
we added are now available in the upstream repo, so have migrated
back.

## Why do we have algod.oas2.json and algod.oas3.yml?

We chose to maintain V2 and V3 versions of the spec because OpenAPI v3 wasn't
widely supported when the V2 API was introduced. Some tools worked better with
V3 and others with V2, so having both available has been useful. **algod.oas2.json
is the hand-authored source of truth**; edit it and run `make generate` to
regenerate everything else. **algod.oas3.yml is a generated artifact** and should
not be edited by hand.

The v2 spec is converted to v3 locally by the `oas2to3` helper (see
`oas2to3/main.go`), which uses [kin-openapi](https://github.com/getkin/kin-openapi)
(already a dependency of this module). Previously this conversion was performed by
the external [converter.swagger.io](http://converter.swagger.io/) service; doing it
locally removes a network dependency from the build and from CI.

kin-openapi handles the structural conversion; `oas2to3` then post-processes
its output to be byte-identical (after canonicalization) to what the
swagger.io converter produced for this spec — restoring dropped vendor
extensions (e.g. `x-go-type`), inlining parameter and response `$ref`s,
expanding response content for each `produces` media type (msgpack),
translating Swagger 2.0 `collectionFormat`, and matching several cosmetic
converter behaviors. This keeps the published spec stable for downstream
consumers and the generated Go code unchanged. If a future spec change relies
on a Swagger 2.0 feature the helper does not yet handle, extend `oas2to3`
accordingly (see the scope note in `oas2to3/main.go`).
