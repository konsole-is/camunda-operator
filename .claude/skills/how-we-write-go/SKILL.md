---
name: how-we-write-go
description: Use when about to write, review, or modify any Go code in this repository — functions, types, controllers, builders, webhooks, or tests.
---

# How We Write Go

## Doc comments

A godoc comment is a contract. A caller must be able to understand the behavior, preconditions, and outcome of using a function, type, or constant without reading its implementation. An inaccurate or misleading godoc is worse than none — it produces incorrect mental models and silent bugs. Accuracy is non-negotiable for maintainability.

**When you touch code that has an inaccurate or unclear doc comment, fix it.** Do not leave it as-is just because it was there before. First ask: is the comment wrong, or is the code wrong? Reconcile accordingly — correct the comment to match the code, or correct the code to match the stated contract, whichever is right.

**Exported identifiers** must have a doc comment. Start with the identifier name. Write as much as clearly and unambiguously describes the contract — no more, no less. For a simple function or constant, one sentence usually suffices; for a type or interface with non-obvious semantics or subtle constraints, a short paragraph is appropriate.

```go
// ComponentName is the label value for this component's app.kubernetes.io/component label.
const ComponentName = "zeebe-analytics"

// Validate reports whether the receiver is in a usable state.
func (a *Analytics) Validate() error { ... }

// Owner is implemented by any resource whose name can be used as the namespace base.
// The returned name must be stable across reconcile loops; it is used to derive
// the managed namespace and all label selectors for the component.
type Owner interface { ... }
```

**Internal identifiers** get no comment unless the behavior would genuinely surprise a reader. Keep it to what a reader actually needs — not a summary of the code below it.

```go
// returns suffix unchanged when base is empty to avoid a leading hyphen
func formatResourceName(base, suffix string) string { ... }
```

**The test for a bad comment:** could a code generator produce it by prepending a verb to the identifier name? If yes, it carries no information beyond the name itself — delete or rewrite it.

- `// ComponentName returns the component name` → generated noise; delete
- `// ComponentName returns the value used in app.kubernetes.io/component labels` → adds information; keep

**Never:**
- Restate what the name already says (apply the code-generator test above)
- Leak implementation context that will rot (`// Typically this will be a *cloudv1alpha1.ZeebeCluster`)
- Add temporal or task context (`// In production this would...`, `// Added for the X flow`)
- Pad a comment to look thorough — every sentence must earn its place

## Inline comments

Write a comment only when the **why** is non-obvious: a hidden constraint, a counter-intuitive invariant, a workaround for a specific upstream bug.

```go
// logr.Logger is nil-safe, but controller-runtime's FromContext returns a discard logger
// when no logger is in context — so this is always safe to call without a nil guard.
logger := log.FromContext(ctx)
```

**Never narrate what the code does:**

```go
// BAD — narrates WHAT
// Build the label set that will be applied to the Deployment.
labels := map[string]string{ ... }

// BAD — AI slop
// In production code this would go through the event recorder.
```

If removing the comment would not confuse a reader six months from now, delete it.

## Whitespace rhythm

Each logical step gets its own block, separated by a blank line. The only exception: a call and its immediate error handler stay together — the error check is the direct continuation of the call, not a new step.

```go
// Each guard checks a different input — separate concerns, separate blocks
if deploymentName == "" {
    return errors.New("analytics: deployment name must not be empty")
}

if obj == nil {
    return errors.New("analytics: object must not be nil")
}

// Fetching: call and error handler are one unit
deployment, err := fetchDeployment(ctx, deploymentName)
if err != nil {
    return fmt.Errorf("fetching deployment %q: %w", deploymentName, err)
}

// Building labels is a separate step from fetching
labels := map[string]string{
    "app.kubernetes.io/component": ComponentName,
    "app.kubernetes.io/name":      deployment.Name,
}

// Applying is a separate step from building
if err := applyLabels(ctx, labels); err != nil {
    return fmt.Errorf("applying labels: %w", err)
}
```

The anti-pattern to avoid is inserting blank lines *within* a logical unit — e.g., between a call and its error handler, or between two lines that together produce one value.

## Line breaking long calls

When a function call does not fit on one line, use judgment. If the function name alone is what makes the line long and the arguments are few or short, breaking after the opening parenthesis and keeping the arguments together is sufficient:

```go
// GOOD — name was the problem, args fit together
someVeryLongReconcilerHelperFunction(
    ctx, deploymentName,
)
```

When arguments are many or individually long, put each on its own line with a trailing comma and close on its own line:

```go
// BAD — crammed onto one line
recorder.Event(obj, corev1.EventTypeNormal, EventReasonReconciled, fmt.Sprintf("deployment %q reconciled", deploymentName))

// BAD — inconsistent split (some args together, some not)
recorder.Event(obj, corev1.EventTypeNormal,
    EventReasonReconciled, fmt.Sprintf("deployment %q reconciled", deploymentName))

// GOOD — one argument per line
recorder.Event(
    obj,
    corev1.EventTypeNormal,
    EventReasonReconciled,
    fmt.Sprintf("deployment %q reconciled", deploymentName),
)
```

The same rule applies to function signatures and struct literals:

```go
// BAD
func ReconcileDeployment(ctx context.Context, deploymentName string, recorder record.EventRecorder, logger logr.Logger) error {

// GOOD
func ReconcileDeployment(
    ctx context.Context,
    deploymentName string,
    recorder record.EventRecorder,
    logger logr.Logger,
) error {

// BAD
return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels}}

// GOOD
return &corev1.ConfigMap{
    ObjectMeta: metav1.ObjectMeta{
        Name:      name,
        Namespace: namespace,
        Labels:    labels,
    },
}
```

## Code bloat

Every line of code is a liability: it has to be read, understood, tested, and maintained. Before writing anything, ask whether it needs to exist at all.

**Use builtins and standard library functions.** Don't reimplement what already exists.

```go
// BAD — manual join
result := ""
for i, s := range items {
    if i > 0 {
        result += ", "
    }
    result += s
}

// GOOD
result := strings.Join(items, ", ")
```

**Grep before you write.** If you are about to write a helper, check whether one already exists in the repo. Writing a second `formatResourceName` two packages away from the first is pure bloat.

**Prefer fewer, simpler constructs.** This applies at every level — conditions, functions, entire code blocks.

Multiple conditions with the same outcome belong in one guard:

```go
// BAD
if a == nil {
    return ErrInvalidInput
}
if b == nil {
    return ErrInvalidInput
}

// GOOD
if a == nil || b == nil {
    return ErrInvalidInput
}
```

A function that wraps a single call and adds nothing is not a function — it is noise:

```go
// BAD — this is just obj.Name with extra steps
func getDeploymentName(obj *appsv1.Deployment) string {
    return obj.Name
}

// GOOD — use obj.Name directly at the call site
```

Code blocks left behind after a refactor often do nothing anymore. When you change code, scan the surrounding context: does any adjacent code now duplicate what you just added, or call something that no longer exists in meaningful form?

```go
// BAD — appendCommonLabels was folded into buildLabels last refactor; this call is now a no-op
labels := buildLabels(name, component)
labels = appendCommonLabels(labels)
applyLabels(obj, labels)

// GOOD
labels := buildLabels(name, component)
applyLabels(obj, labels)
```


## Function roles in a controller

A controller reconciler is a layered system. Each layer has a defined responsibility; mixing them creates invisible coupling and makes bugs hard to trace. The rules are not rigid constraints but a structure to reason about — the key is that every function's role is clear and consistent.

**Layer responsibilities; do not conflate them.** A large controller will have sub-reconcilers, each owning a specific resource or concern — and each of those may write to the API server. That is fine. The convention is not "only one function ever writes" but "each layer has a clear, defined responsibility and does not silently do the work of another layer." A helper that mutates an in-memory object should not also persist it. A sub-reconciler that owns a Deployment writes that Deployment; it does not also patch the parent resource.

A function that does more than its name and signature suggest is impure: it produces side effects the caller cannot see, cannot control, and cannot test in isolation. A function named `applyLabels` that also calls `client.Update` does two things while advertising one.

```go
// BAD — helper persists changes the caller doesn't know about
func (r *Reconciler) applyLabels(ctx context.Context, obj *appsv1.Deployment) error {
    obj.Labels = buildLabels(obj)
    return r.client.Update(ctx, obj) // hidden write — caller has no control over this
}

// GOOD — helper mutates in-memory; the layer that owns the write decides when to persist
func applyLabels(obj *appsv1.Deployment) {
    obj.Labels = buildLabels(obj)
}

// In the owning layer:
applyLabels(deployment)
// ... other mutations ...
if err := r.client.Update(ctx, deployment); err != nil {
    return ctrl.Result{}, fmt.Errorf("updating deployment: %w", err)
}
```

**Signal mutation in the name or doc comment.** A function that modifies a passed object must make that obvious — either through a verb like `set`, `apply`, `mutate`, or an explicit doc comment. A reader should not have to trace into the body to discover that the object was changed.

```go
// clear from the name
func setOwnerReference(obj metav1.Object, owner metav1.Object) { ... }
func applyNetworkPolicy(deployment *appsv1.Deployment, policy NetworkPolicy) { ... }

// or clear from the doc comment when the name alone isn't enough
// configureProbes sets liveness and readiness probes on the container spec
// according to the component's health check settings.
func configureProbes(container *corev1.Container, cfg HealthConfig) { ... }
```

**Only the top level returns `ctrl.Result`.** Helpers and sub-functions return `error` or domain types. The reconciler is the single point that decides whether to requeue, after how long, and with what context. Spreading `ctrl.Result` returns across multiple layers produces conflicting requeue decisions and makes the control flow impossible to follow.

```go
// BAD — helper makes requeue decisions
func (r *Reconciler) ensureDeployment(ctx context.Context, obj *myv1.MyResource) (ctrl.Result, error) {
    if notReady {
        return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
    }
    return ctrl.Result{}, nil
}

// GOOD — helper returns an error or a sentinel; reconciler decides what to do
var ErrNotReady = errors.New("deployment not ready")

func ensureDeployment(ctx context.Context, obj *myv1.MyResource) error {
    if notReady {
        return ErrNotReady
    }
    return nil
}

// In the reconciler:
if err := ensureDeployment(ctx, resource); errors.Is(err, ErrNotReady) {
    return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
} else if err != nil {
    return ctrl.Result{}, err
}
```

## Typed string constants

Event reasons, condition types, label keys, annotation keys, and any string that crosses a package boundary must be declared as constants, not written inline.

For the event type argument to `recorder.Event`, always use the framework constants `corev1.EventTypeNormal` and `corev1.EventTypeWarning` — never the raw strings `"Normal"` or `"Warning"`.

```go
// BAD
recorder.Event(obj, "Normal", "SuccessfulReconcile", "...")
recorder.Event(obj, "Warning", "ProvisionFailed", "...")

// GOOD
const EventReasonReconciled = "Reconciled"

recorder.Event(obj, corev1.EventTypeNormal, EventReasonReconciled, "deployment reconciled")
recorder.Event(obj, corev1.EventTypeWarning, EventReasonProvisionFailed, "failed to provision deployment")
```

Prefer a custom `type Reason string` in packages that own many event reasons, so the compiler catches misuse.

## Controller: events vs logs

| Signal | Use for | Tool |
|--------|---------|------|
| **Event** | User-visible state transitions on a specific object (provisioning started, health check failed, config applied) | `recorder.Event(obj, type, reason, msg)` |
| **Log** | Operator-internal tracing, debugging, diagnostic detail | `logger.Info(...)` / `logger.Error(...)` |

Rules:
- Prefer events for anything a cluster operator would want to `kubectl describe` and understand without reading operator logs.
- Keep `logger.Info` calls sparse in the reconcile hot path — every reconcile of every object emits them; they bloat the log stream.
- Do not log and record an event for the same fact. Pick the right signal.
- Do not log errors that are returned from the reconciler. Controller-runtime logs them automatically; logging again produces duplicate entries and inflates the noise. If you return `ctrl.Result{}, err`, do not also call `logger.Error(err, ...)`.
- Use `logger.Error` only for errors that are explicitly swallowed — i.e., errors you handle and do not return. If you return the error, let the framework log it.
- Use `logger.V(1)` or higher for debug-level detail; leave `V(0)` (the default) for genuinely important state changes.

```go
// BAD — log masquerading as an event, event reason is freeform
logger.V(1).Info("recording event", "reason", "SuccessfulReconcile")

// GOOD — real event, named constant reason
recorder.Event(obj, corev1.EventTypeNormal, EventReasonReconciled, "deployment reconciled")
```

## Error wrapping

Always wrap with `%w` to preserve the chain. Include enough context to identify the call site without a stack trace:

```go
// BAD
return err
return fmt.Errorf("failed: %v", err)

// GOOD
return fmt.Errorf("reconciling analytics deployment %q: %w", name, err)
```

The prefix should read as a stack of calls: `"outer: inner: leaf error"`.

## Exported vs internal: how much context to give

| Identity | Exported? | Doc comment? | Detail level |
|----------|-----------|--------------|--------------|
| Package-level const/var | yes | required | name-first; as long as the contract needs |
| Exported type | yes | required | what it represents, not its fields |
| Exported method/func | yes | required | what it does, not how; length matches complexity |
| Interface | yes | required | what the implementor promises (not typical implementors) |
| Internal func/method | no | only if surprising | WHY, not WHAT |
| Internal type/const | no | almost never | |

## CRD fields: validation and pointer decisions

When adding a new field to a CRD type, stop and ask the following questions before writing any code.

### What are the constraints?

Work out the full constraint set before choosing how to enforce it:

- Is the field required or optional?
- Does it have a minimum or maximum value / length / item count?
- Are there mutually exclusive fields (set A or B, not both)?
- Is one field conditionally required based on another?
- Is the value constrained to an enumerated set?

### Validation hierarchy — use the first option that fits

**1. Kubebuilder markers** — prefer these for single-field constraints. They are declarative, generate CRD schema automatically, and are validated by the API server before a request reaches the controller.

```go
// +kubebuilder:validation:Required
// +kubebuilder:validation:Minimum=1
// +kubebuilder:validation:Maximum=100
Replicas int32 `json:"replicas"`

// +kubebuilder:validation:Optional
// +kubebuilder:validation:MinItems=1
// +kubebuilder:validation:MaxItems=10
Zones []string `json:"zones,omitempty"`

// +kubebuilder:validation:Optional
// +kubebuilder:validation:Enum=Retain;Delete
ReclaimPolicy string `json:"reclaimPolicy,omitempty"`
```

**2. CEL validation (`+kubebuilder:validation:XValidation`)** — use for cross-field constraints that kubebuilder markers cannot express: mutual exclusivity, conditional requirements, relational checks. CEL runs in the API server, so it still requires no webhook.

```go
// Exactly one of StorageClass or VolumeClaimTemplate must be set, not both.
// +kubebuilder:validation:XValidation:rule="has(self.storageClass) != has(self.volumeClaimTemplate)",message="exactly one of storageClass or volumeClaimTemplate must be set"
type StorageSpec struct {
    // +optional
    StorageClass *string `json:"storageClass,omitempty"`
    // +optional
    VolumeClaimTemplate *corev1.PersistentVolumeClaimSpec `json:"volumeClaimTemplate,omitempty"`
}
```

CEL patterns to know:
- `has(self.field)` — field is present (for optional fields)
- `self.field.size() >= 1` — non-empty string or list
- `has(self.a) != has(self.b)` — mutual exclusivity
- `!has(self.a) || self.a == ''` — field absent or empty

**3. Webhook validation** — last resort, for constraints that require external state, complex logic, or error messages CEL cannot produce cleanly. Webhook code is harder to test and adds operational complexity. Exhaust kubebuilder markers and CEL first.

### Pointer vs non-pointer fields

The decision is about **whether the zero value is meaningful** and **whether you need to distinguish "not set" from "set to zero"**.

**`Enabled bool` on an optional feature struct is almost always wrong.** An `Enabled bool \`json:"enabled,omitempty"\`` field drops `false` under `omitempty`, making "explicitly disabled" indistinguishable from "field not set". Unless `false` is the unambiguous and permanent default — i.e., there is no scenario where an operator needs to distinguish "not configured" from "set to off" — use `*bool`.

Use a **pointer (`*T`)** when:
- The field is optional and the zero value of `T` is a valid, meaningful value that must be distinguishable from "not configured". The clearest case is `*bool`: a bare `bool` cannot represent three states (true / false / not set), and `false` is silently dropped by `omitempty`.
- You need to detect whether the user explicitly provided the field at all (e.g., to apply a default only when absent).

```go
// *bool is required — false and "not set" are different things
// +optional
Enabled *bool `json:"enabled,omitempty"`

// *int32 when 0 is a valid value distinct from "unset"
// +optional
TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`
```

Use a **non-pointer (`T`)** when:
- The field is required — a nil check adds nothing and the zero value is never legitimate.
- The zero value is a safe, unambiguous default (e.g., an empty string that the controller treats as "use the default", or a `bool` that only needs to represent on/off with no need to detect "not configured").
- The type is already a reference type (slices, maps, interfaces) — wrapping them in a pointer rarely adds clarity and adds a double-dereference burden.

```go
// Required — no pointer needed
// +kubebuilder:validation:Required
Name string `json:"name"`

// Slice is already a reference type; nil and empty are handled by omitempty
// +optional
Tags []string `json:"tags,omitempty"`
```

## Naming

**Use the shortest name that is unambiguous in its scope.** A name should say what a thing is, not repeat where it came from or what type it is. Type information is already in the declaration; suffixing it into the name is noise.

```go
// BAD — suffixes add no information
backupCR        // it's a Backup; CR is obvious from context
deploymentObj   // it's a *appsv1.Deployment; Obj adds nothing
reconcileReq    // it's a ctrl.Request; Req is already obvious

// GOOD
backup
deployment
req
```

**No stuttering.** If the package is `backup`, the primary type does not need `Backup` in its name at the call site — `backup.Resource` reads fine, `backup.BackupResource` repeats itself.

**Acronyms are all-caps.** `HTTPClient`, `userID`, `parseURL` — not `HttpClient`, `userId`, `parseUrl`.

**Short names are appropriate in short scopes.** `i`, `v`, `k`, `err` are idiomatic in loops and brief blocks. Use more descriptive names at package scope or in functions where the variable lives far from its declaration.

**Receiver names** are one or two letters, consistent across all methods of a type, and never `self` or `this`.

```go
// BAD
func (self *Analytics) Validate() error { ... }
func (a *Analytics) ComponentName() string { ... } // inconsistent with above

// GOOD — pick one and use it everywhere on the type
func (a *Analytics) Validate() error { ... }
func (a *Analytics) ComponentName() string { ... }
```

## Context propagation

`context.Context` is always the first parameter of any function that does I/O or calls into the Kubernetes API. It is always named `ctx`. It is never stored in a struct — a stored context outlives the request it belongs to, bypasses cancellation, and makes the function's dependencies invisible.

```go
// BAD — context stored in struct
type Reconciler struct {
    ctx    context.Context
    client client.Client
}

// GOOD — context flows through the call chain as an explicit parameter
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    obj, err := r.fetchObject(ctx, req.NamespacedName)
    if err != nil {
        return ctrl.Result{}, fmt.Errorf("fetching object: %w", err)
    }
    return r.reconcileDeployment(ctx, obj)
}
```

Pass `ctx` to every function that touches the API server, writes a log entry, or calls an external service. Do not create a new `context.Background()` mid-call — that discards the caller's deadline and cancellation signal.

## Interface design

Define interfaces in the package that consumes them, not the package that implements them. Go's implicit satisfaction means implementations do not need to know about the interface; the consumer declares exactly the contract it needs.

Keep interfaces as small as feasible — declare only the methods the consumer actually needs. An interface that requires more than the consumer uses forces implementors to satisfy a broader contract than necessary, and usually signals that the abstraction is wrong.

Accept interfaces, return concrete structs. Callers can always widen a concrete return to an interface; narrowing in the other direction requires a type assertion.

Do not create an interface speculatively. Create one when you have two real implementations or a genuine need to test against a substitute. An interface with one implementation that never changes is indirection for its own sake.

```go
// BAD — defined next to the implementation, large, speculative
// in pkg/apps/analytics/analytics.go:
type AnalyticsInterface interface {
    Reconcile(ctx context.Context) error
    Validate() error
    ComponentName() string
    SetLabels(labels map[string]string)
    GetOwner() Owner
}

// GOOD — defined at the consumer, declares only what the consumer needs
// in internal/controller/cloud/zeebecluster_controller.go:
type componentReconciler interface {
    Reconcile(ctx context.Context) error
}
```

**Do not wrap existing controller-runtime interfaces.** This guideline is about creating new interfaces where none exists. When controller-runtime already provides an interface that covers what you need — `client.Object`, `client.Client`, `record.EventRecorder` — use it directly. Do not re-declare methods from it into a new interface. Re-declaring `GetName() string` and `GetNamespace() string` manually produces an interface that satisfies your contract but not the framework's, causing type errors when the value is passed to any controller-runtime API (`recorder.Event`, `client.Get`, etc.) that takes `client.Object` or `runtime.Object`.

```go
// BAD — re-declares methods already on client.Object; won't satisfy recorder.Event
type Owner interface {
    GetName() string
    GetNamespace() string
}

// GOOD — use client.Object directly when the value will be passed to framework APIs
func NewBackupReconciler(owner client.Object, ...) *BackupReconciler
```

## Status conditions

Use `meta.SetStatusCondition` to write conditions — never append or assign directly to the slice. It handles deduplication and sets `LastTransitionTime` only when the status changes.

**`meta.SetStatusCondition` only mutates the in-memory slice.** It does not persist anything on its own. Every controller persists status through the ocf `component.FlushStatus`, once per reconcile, deferred at the top of the reconcile loop. That holds for a controller with no components too: stage the condition, set `observedGeneration`, flush. There is no second write path — no SSA of the status subresource, no `Status().Update()` at early returns.

```go
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
    var obj v1.Database
    if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    rec := component.ReconcileContext{Client: r.componentClient, Scheme: r.Scheme, Recorder: r.Recorder, Owner: &obj}
    defer func() {
        if flushErr := component.FlushStatus(ctx, rec); flushErr != nil {
            err = errors.Join(err, flushErr)
        }
    }()

    // Stage conditions freely with meta.SetStatusCondition; the defer writes them.
    ...
}
```

The named return `(_ ctrl.Result, err error)` lets the deferred flush join its error onto `err`, even when the reconcile itself succeeded. `FlushStatus` retries on 409 and re-merges the staged conditions by type.

**Condition vocabulary is API surface and lives in `api/v1`.** Users match it with `kubectl wait`, and the operators above import `api/v1` to gate on it. `api/v1/conditions.go` holds the vocabulary that more than one CRD reports (`ConditionReady`, `ReasonHealthy`, `ReasonProgressing`, `ReasonInvalidReference`, `ReasonMissingSecret`, `ReasonConnectionFailed`, `ReasonSuspended`). A reason or condition type that one CRD reports is declared in that CRD's types file, next to its spec. The CRD doc under `docs/crds` is the contract for both. `pkg/conditions` derives the aggregate `Ready`; it declares no vocabulary.

```go
// api/v1/elasticsearchcluster_types.go
// ConditionSuspended reports whether the node set is scaled to zero by spec.suspend.
const ConditionSuspended = "Suspended"
```

Always set `ObservedGeneration` to `obj.Generation` — it lets consumers know whether the condition reflects the current spec revision.

**Keep the condition count low.** Do not add a new condition type for every failure mode or sub-state. Instead, use `Reason` to distinguish states within an existing condition. A `Ready` condition with reasons `Reconciled`, `ProvisionFailed`, and `DependencyNotReady` is cleaner and easier to consume than three separate conditions that all represent aspects of the same concern.

A new condition type is warranted only when a genuinely distinct component or concern needs independent external exposure — for example, a backup subsystem that consumers need to observe separately from overall readiness. When in doubt, use a reason within an existing condition rather than introducing a new one.

**Conditions vs events:** a condition represents the current state of the object (it is idempotent, overwritten each reconcile). An event represents something that happened at a point in time. Use a condition when you want `kubectl get` to show the state; use an event when you want `kubectl describe` to show history.

## Reconcile requeue patterns

The choice between returning an error and returning a requeue result is not just about timing — it is about whether the situation is genuinely unexpected.

**Return an error** when something failed in a way that should be surfaced. Controller-runtime will log the error, increment error metrics, and requeue with exponential backoff. This is the right signal for real failures.

**Return a requeue result with `nil` error** when the situation is expected — a dependency not yet ready, an external resource still provisioning, a condition being waited on. Returning an error here produces noise in logs and false-positive error metrics that obscure real problems.

| Return | When to use |
|--------|-------------|
| `ctrl.Result{}, nil` | Reconcile complete; no further action needed until the next watch event. |
| `ctrl.Result{}, err` | Genuine failure; controller-runtime logs it, increments error metrics, and requeues with exponential backoff. |
| `ctrl.Result{RequeueAfter: d}, nil` | Expected intermediate state — waiting on something. No error logged, no backoff, retries after `d`. Record an event so the user can see why the object is waiting. |
| `ctrl.Result{Requeue: true}, nil` | Immediate retry without error. Prefer `RequeueAfter` with a short duration to avoid spinning. |

Do not return both a non-zero `Result` and a non-nil `error` — the error takes precedence and the `Result` is ignored by controller-runtime.

Do not use `RequeueAfter` as a workaround for missing watch events. If you own the resource being waited on, add a watch instead. `RequeueAfter` is for external systems you cannot watch.

## Panic vs error

Never call `panic` in a reconcile loop or any function reachable from it. A panic in a controller goroutine crashes the manager process and takes every other controller down with it. All runtime errors must be returned as `error`.

`panic` (via `Must`-prefixed constructors) is appropriate only for programmer errors caught at init time — configuration that is structurally impossible to get right at runtime:

```go
// OK — panics at startup if the constraint string is malformed; caught immediately
var FeatureX = feats.MustRegister("Feature X", ">= 8.8.0")

// NEVER — panics during a live reconcile
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    cfg := mustLoadConfig() // if this panics, the entire manager dies
    ...
}
```

If you find yourself wanting to panic at runtime because "this should never happen", return an error with enough context to diagnose it instead.

## Where to put code

File and package organization follows concern, not type. When a group of related functions grows beyond what fits naturally in one file, give it its own file named after the concern it serves — not after the type it operates on.

**When a concern earns its own file, split it out — and make the split complete.** If enqueue logic, a sub-reconciler, or any other coherent set of functions has grown distinct enough to justify a separate file, move it to that file and pull in any related code that already exists elsewhere in the package and belongs to the same concern. A partial split — some of the concern in the new file, the rest left behind — is worse than no split at all because it scatters the concern across files without making either file coherent.

Name the file after the concern it holds, not after the type it operates on. `backup_reconciler.go` is clearer than `zeebeclusterbackup_reconciler.go`. Enqueue and watch logic, for instance, naturally belongs in its own file rather than cluttering the main controller file, and any predicate or index registration that belongs to the same watch setup should live there too.

```
internal/controller/database/
  controller.go   ← Reconcile(), pre-checks and other I/O, watches, SetupWithManager
  suite_test.go   ← the envtest suite of this package, through internal/testenv
```

**Controller packages hold I/O and wiring. Everything that maps a spec to resources belongs in `pkg/`.** A controller package under `internal/controller/<crd>/` contains `Reconcile`, the pre-checks and other calls against the API server or external systems, the watches, the indexes, and `SetupWithManager`. It does not contain builders.

Put the following in `pkg/components/<crd>/`:

- Resolution of the documented defaults (names, namespaces, derived identifiers)
- Naming rules that a user or another operator can observe
- ocf component assembly and resource builders
- Pure rules that the reconcile applies, for example a collision rule or a preset merge

```
pkg/components/database/             ← ResolveBindings, BackupUserName, BindingsComponent, collision rule
pkg/components/elasticsearchcluster/ ← MergePreset, ValidateMerged, the three components
pkg/pgbootstrap/                     ← SQL layer
pkg/credentials/, pkg/conditions/    ← shared primitives
```

The package is pure: spec in, resources out, no API calls. Its golden tests live next to it and run without envtest. The controller imports it with the alias `components`, so `components.ResolveBindings(&database)` reads the same in every controller.

A single consumer is not a reason to keep a builder in the controller. The next operator up the stack must be able to predict the names and shapes this operator publishes, and it cannot import `internal/`. Move a builder to `pkg/` when you write it, not when a second consumer appears.

Most `pkg/` code stays API-free: use it for construction, transformation, and evaluation, and keep `Get`/`Update`/`Create`/`Patch` calls in controller packages. If a `pkg/` package must touch the API, for example a small retrieval helper, keep it narrow and name it for what it does.

**Webhook logic stays in the webhook package.** Defaulting and validation logic belongs in `internal/webhook/`, not in the controller. The controller should not replicate webhook defaults, and the webhook should not perform reconcile-style API calls.

## Common mistakes

| Mistake | Fix |
|---------|-----|
| Comment restates the name | Delete the comment |
| "Typically this will be a X" in an interface doc | Delete; if you must constrain, use a constraint type |
| Multi-sentence doc where one sentence would be complete | Trim to what the contract actually requires |
| `// In production code this would...` | Delete; write the real code or a `// TODO(#NNN)` |
| Inline `const reason = "..."` in a function | Promote to package-level typed constant |
| `logger.Info` for every reconcile step | Trim to the one line that matters; use events for state changes |
| `fmt.Sprintf("%s-%s", a, b)` | `a + "-" + b` |
