// MLSE-COMPILE: formal
// CHECK-LABEL: func.func @demo.push(
// CHECK-SAME: %dataList: !go.ptr<!go.slice<!go.slice<i8>>>
// CHECK-SAME: %raw: !go.slice<i8>
// CHECK: = go.load %dataList : !go.ptr<!go.slice<!go.slice<i8>>> -> !go.slice<!go.slice<i8>>
// CHECK: = go.append %{{[A-Za-z0-9_%.]+}}, %raw : (!go.slice<!go.slice<i8>>, !go.slice<i8>) -> !go.slice<!go.slice<i8>>
// CHECK: go.store %{{[A-Za-z0-9_%.]+}}, %dataList : !go.slice<!go.slice<i8>> to !go.ptr<!go.slice<!go.slice<i8>>>
// CHECK-NOT: @demo.append
package demo

func push(dataList *[][]byte, raw []byte) {
	*dataList = append(*dataList, raw)
}
