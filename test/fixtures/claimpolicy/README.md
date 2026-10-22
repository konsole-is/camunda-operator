# Claim policy fixtures

The cases in this directory show the policy rules that both claim controllers apply. The functions of `pkg/claimpolicy` hold these rules, and their table tests run each case. The fleet manager runs its own policy code against the cases of `environment-of`, `resolve`, `check-allowed`, `inputs`, `slot-fields`, and `check-slot-conflicts`. The rules of `order` and `target-service-account` have no page in the fleet manager, so it does not run their cases.

## Layout

There is one directory for each function of `pkg/claimpolicy`:

| Directory | Function |
| --- | --- |
| `environment-of/` | `EnvironmentOf` |
| `resolve/` | `Resolve` |
| `check-allowed/` | `CheckAllowed` |
| `order/` | `Order` |
| `inputs/` | `Inputs` |
| `target-service-account/` | `TargetServiceAccount` |
| `slot-fields/` | `SlotFields` |
| `check-slot-conflicts/` | `CheckSlotConflicts` |

## Format

Each `<case>.json` file holds one case:

```json
{
  "schemaVersion": 1,
  "name": "A label that names no Environment is refused",
  "input": {},
  "result": {
    "refusal": {
      "reason": "UnknownEnvironment",
      "message": "namespace \"team-a\" has the label fleet.camunda.io/environment=staging, and no Environment \"staging\" exists"
    }
  }
}
```

- `schemaVersion` is the version of this format. It changes only when the format changes.
- `name` says what the case shows.
- `input` holds the arguments of the function. The field names are the field names of the API. A wrong name or a wrong case makes the test fail.
- `result` holds exactly one of three fields. `value` is the output of the function. `refusal` is a refusal, with its `reason` and its `message`. `error` is any other error, as text. Do not write `result` by hand.

A reader in another language compares `value` and the `reason` of a `refusal`. The texts of `message` and `error` are the texts of the Go code.

## Add a case

1. Write the file without `result`.
2. Run `make claimpolicy-fixtures`. It writes `result` into every case and formats each file.
3. Read `result`. If it is not what the rule says, the case or the code is wrong. Do not commit it.
4. Commit the case.

A new directory also needs a place in `APP_RULES` or in `EXCLUDED` of `fleet-manager/test/claimpolicy/fixtures.ts`. Otherwise the fleet manager tests fail.

## Change a rule

When you change a function of `pkg/claimpolicy`, run `make claimpolicy-fixtures` and commit the changed cases. The Lint workflow fails when the cases differ from the output of `make claimpolicy-fixtures`.

Each refusal reason needs at least one case, and each directory needs a table test. The tests fail when one of them is missing.
