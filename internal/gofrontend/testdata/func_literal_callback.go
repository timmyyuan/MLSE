// CHECK-LABEL: func.func @demo.main()
// CHECK: %[[CALLBACK:[A-Za-z0-9_%.]+]] = func.constant @demo.main.__lit0 : () -> ()
// CHECK: func.call @demo.run(%[[CALLBACK]]) : (() -> ()) -> ()
// CHECK-LABEL: func.func @demo.run(%call: () -> ())
// CHECK: func.call_indirect %call() : () -> ()
// CHECK-LABEL: func.func private @demo.main.__lit0()
// CHECK: func.call @demo.ping() : () -> ()
package demo

func ping() {}

func run(call func()) {
	call()
}

func main() {
	run(func() {
		ping()
	})
}
