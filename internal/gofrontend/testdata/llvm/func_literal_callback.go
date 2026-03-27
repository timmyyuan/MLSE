// MLSE-COMPILE: default
// LLVM-LABEL: define{{.*}} void @demo.main()
// LLVM: call void @demo.run(ptr @demo.main.__lit0)
// LLVM-LABEL: define{{.*}} void @demo.run(ptr %0)
// LLVM: call void %0()
// LLVM-LABEL: define void @demo.main.__lit0()
// LLVM: call void @demo.ping()
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
