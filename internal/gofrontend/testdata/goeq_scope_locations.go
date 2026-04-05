// CHECK-LABEL: func.func @demo.check(%err: !go.error, %name: !go.string) -> i1 attributes {go.scope = 0 : i64} {
// CHECK: %nil{{[0-9]+}} = go.nil : !go.error loc("scope1"("testdata/goeq_scope_locations.go":
// CHECK: %cmp{{[0-9]+}} = go.neq %err, %nil{{[0-9]+}} : (!go.error, !go.error) -> i1 loc("scope1"("testdata/goeq_scope_locations.go":
// CHECK: %if{{[0-9]+}} = scf.if %cmp{{[0-9]+}} -> (i1) {
// CHECK: %str{{[0-9]+}} = go.string_constant "" : !go.string loc("scope1"("testdata/goeq_scope_locations.go":
// CHECK: %cmp{{[0-9]+}} = go.neq %name, %str{{[0-9]+}} : (!go.string, !go.string) -> i1 loc("scope1"("testdata/goeq_scope_locations.go":
// CHECK: } loc("scope1"("testdata/goeq_scope_locations.go":
package demo

func check(err error, name string) bool {
	ok := false
	if err != nil {
		ok = name != ""
	}
	return ok
}
