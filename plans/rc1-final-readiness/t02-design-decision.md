# T02 scope decision — owner selected B

T02 is blocked. The current prototype does not meet the approved lifetime/persistence acceptance criteria. No runtime acceptance or publication is authorized by this note.

**Owner decision:** selected **B: Expand runtime scope** through the interactive scope question. Proceed with coordinated design across anduin/inertia/kiks; do not implement option A's limitations or option C's waiver. Publication/tagging remains unauthorized. Implementation must wait for a concrete enforceable error-lifecycle design; any unavoidable compatibility change must be stated explicitly rather than hidden.

## Why a decision is needed

Rendering before middleware unwinds conflicts with existing error-consumption behavior and inertia's response capture: an outer middleware can consume an error and supply its own response; inertia discards its capture when an error returns. Rendering early can therefore duplicate or discard responses. Retaining the deepest context and rendering later instead exposes canceled child contexts and already-released resources. A repeated-next attribution bug is also confirmed.

## A — narrow anduin-only fix (recommended)

Change the approved T02 contract explicitly:

- Keep beta.7 render timing: route/handler errors after route middleware unwinds; host/global errors after the global chain unwinds. Preserve original error inspection/consumption and exactly-once rendering.
- Read request values from the deepest context of the latest failing pass, but derive cancellation/deadlines/cause from the live flush-point context. Use standard context primitives so hidden cancellation keys cannot point at the canceled child. Test `Err`, `Cause`, deadlines, disconnect cancellation, SSR requests and cleanup under race.
- Fix repeated-next attribution; test fail A/fail B and fail A/succeed B.
- Explicitly document/test that middleware-scoped resources may already be closed. Retaining a value is not extending a transaction/connection/lock lifetime; error providers must not use released resources.
- Explicitly document/test that host/global error renderers may **read** the kiks bag, but changes to that bag after its save do not persist. Flash status classification remains based on the pre-render status. Route-level persistence needs separate positive coverage. Direct response cookies remain possible.
- No inertia/kiks changes or extra package releases. Keep their broader persistence follow-up out of rc.1. Do not claim the original correct-time persistence guarantee is met.

First inspect the site's actual host-error providers: if they mutate session/flash rather than only read authentication, option A does not fully satisfy that use case and must be reconsidered. Local and published integration must include a canceling middleware, not only the original happy-path context repro.

## B — expand scope to resource-aware early rendering

Design coordinated anduin/inertia/kiks behavior, including capture commit, consumed errors, persistence timing and middleware error observation. Requires additional module releases and renewed acceptance. No existing implementation is approved for this contract.

## C — defer B7-005 with explicit rc.1 waiver

Exclude the unaccepted runtime prototype from the candidate, retain the site's tested anonymous-fallback workaround, document loss of signed-in chrome and known risk. Continue with the other three fixes. This changes the approved four-fix quality bar and requires an owner waiver.

## Recommendation and controls

Choose A only with explicit acceptance of the released-resource and host/global bag-write limitations; choose B if those guarantees are required for rc.1. C is available if runtime scope should not expand. No tagging, publishing, or final readiness declaration while this decision is open.
