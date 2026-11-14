# Nullable Oracle `PLS_INTEGER` across gRPC, ProtoJSON, and SOAP

## Opt-in (default: disabled)

Run the main `oracall` generator with `-nullable-pls-integer`:

```sh
oracall -nullable-pls-integer -db-out ./pkg/db -pb-out ./pkg/pb ...
```

Without the flag, `PLS_INTEGER` / `BINARY_INTEGER` generation remains exactly
as before: `sint32`/`int32` fields and legacy Go bindings. The change is a
**generation-time switch**, not a runtime flag; the generated `.proto`, `.pb.go`
and DB wrappers must be rebuilt together. It also affects API semantics:
legacy clients that omit zero-valued fields may now be interpreted as NULL.

Code using the `lib` / `lib/objects` generator APIs directly (rather than the
main CLI) can set `oracall.NullablePLSInteger = true` before generating. The
setting is process-global, so do not mutate it concurrently with generation.
The `custom.ToSQLInt32`, `custom.FromSQLInt32`, and `custom.SOAPInt32` helpers
are always available, even with the flag off, but generation does not emit calls
to the SQL helpers unless it is enabled.

Scalar `PLS_INTEGER` and `BINARY_INTEGER` use `optional sint32` in the legacy
generated proto3 API. The newer object-generator uses `optional int32` because
its existing wire type is `int32` and changing it to `sint32` would break clients.
Both have generated Go fields of `*int32` (with the Open Struct API),
while Oracle's `godror` binding uses `sql.NullInt32` internally. Zero and `NULL`
therefore remain distinct for `IN`, `OUT`, and `IN OUT` parameters and for
scalar attributes in Oracle records/objects.

| Oracle | protobuf field | ProtoJSON | SOAP (nillable XML Schema element) |
|---|---|---|---|
| `NULL` | absent / nil pointer | omitted (also accepts explicit `null` on input) | `<count xsi:nil="true"/>` |
| `0` | present, pointer to zero | `"count": 0` | `<count>0</count>` |
| `-17` | present, pointer to -17 | `"count": -17` | `<count>-17</count>` |

## Go / database boundary

```go
v := custom.ToSQLInt32(request.PCount)
_, err := db.ExecContext(ctx, `BEGIN pkg.proc(:count); END;`,
    sql.Out{Dest: &v, In: true})
if err != nil { return err }
response.PCount = custom.FromSQLInt32(v)
```

For `IN`-only bindings pass `custom.ToSQLInt32(request.PCount)`. For `OUT`-only
bindings initialize an empty `sql.NullInt32` and pass `sql.Out{Dest: &v}`.

## JSON-RPC / ProtoJSON

For a protobuf message, use `protojson.Unmarshal` and `protojson.Marshal`
(instead of `encoding/json` on the protobuf struct):

```go
var request pb.CallInput
if err := protojson.Unmarshal(jsonRPCParams, &request); err != nil { return err }
jsonRPCResult, err := protojson.Marshal(&response)
```

By default ProtoJSON omits absent optional fields. A JSON `null` value is
accepted on input, but serializing an absent optional field does not emit an
explicit JSON `null`. A JSON-RPC contract that requires explicit null must
implement that representation in its gateway adapter.

## SOAP / XML Schema

There is no SOAP transport implementation in `oracall`, so this change provides
an explicit reusable XML adapter rather than guessing an external gateway's
WSDL or SOAP-envelope conventions:

```go
type CallResponseXML struct {
    Count custom.SOAPInt32 `xml:"count"`
}

xml.Marshal(CallResponseXML{Count: custom.SOAPInt32{Value: response.PCount}})
```

The corresponding WSDL/XSD element must be declared with
`minOccurs="0" nillable="true" type="xs:int"`. `SOAPInt32.UnmarshalXML`
understands `xsi:nil`; `SOAPInt32.MarshalXML` emits it for NULL. It can be used
for SOAP 1.1 or 1.2 envelopes because the `xsi:nil` convention is an XML Schema
feature, not tied to the envelope version.

## Migration / limitations

* Adding presence to an existing `sint32` field retains its scalar wire type,
  but it changes the semantic meaning of an omitted field. Old gRPC clients
  that sent `0` may have omitted the field, so roll out carefully.
* Neither `optional` nor protobuf repeated numeric fields preserve NULL for
  individual array elements. That requires a repeated nullable **message**.
* Omission versus an explicit Oracle `NULL` versus a PL/SQL argument `DEFAULT`
  are three states. `optional` only provides two; use a `oneof` or a separate
  operation to represent all three.
* `NumberAsString` is an existing generator mode and is not converted to
  `optional sint32` by this change.
