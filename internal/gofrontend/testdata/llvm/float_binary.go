// MLSE-COMPILE: formal
// LLVM-LABEL: define { ptr, i64 } @demo.format(i64 %0)
// LLVM: sitofp i64 %0 to double
// LLVM: fdiv double
// LLVM: call ptr @runtime.any.box.f64(double
// LLVM: call { ptr, i64 } @runtime.fmt.Sprintf({ ptr, i64 }
package demo

import "fmt"

func format(n int64) string {
	return fmt.Sprintf("%.1fK", float64(n)/1000)
}
