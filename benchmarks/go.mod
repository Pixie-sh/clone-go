module github.com/pixie-sh/clone-go/benchmarks

go 1.25.1

require (
	github.com/huandu/go-clone/generic v1.7.3
	github.com/pixie-sh/clone-go v0.0.0
)

require github.com/huandu/go-clone v1.7.3 // indirect

replace github.com/pixie-sh/clone-go => ../
