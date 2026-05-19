// CHECK-LABEL: func.func @testdata.F(
// CHECK: %args{{[0-9]+}} = go.make_slice %argc{{[0-9]+}}, %argc{{[0-9]+}} : i64 to !go.slice<!go.named<"any">>
// CHECK: func.call @runtime.any.box.string
// CHECK: func.call @runtime.any.box.error
// CHECK: func.call @testdata.warn(%{{[^,]+}}, %{{[^,]+}}, %args{{[0-9]+}}) : (!go.named<"context.Context">, !go.string, !go.slice<!go.named<"any">>) -> ()
// CHECK-NOT: func.call @testdata.warn(%{{[^)]*}}, %{{[^)]*}}, %{{[^)]*}}, %{{[^)]*}})
// CHECK-LABEL: func.func @testdata.warn(
// CHECK-SAME: !go.slice<!go.named<"any">>
package testdata

import "context"

func warn(_ context.Context, _ string, _ ...any) {}

func F(ctx context.Context, region string, err error) {
	warn(ctx, "region %v err %v", region, err)
}
