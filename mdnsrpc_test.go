package mdnsrpc

import (
	"fmt"
)

type Calculator struct{}

type Numbers []int

func (self *Calculator) Add(numbers Numbers, result *int) (err error) {
	for _, num := range numbers {
		*result += num
	}
	return
}

func ExampleLookupOne() {
	calc := &Calculator{}
	if _, err := Publish("Calculator", calc); err != nil {
		panic(err)
	}
	cli, err := LookupOne("Calculator")
	if err != nil {
		panic(err)
	}
	res := 0
	if err := cli.Call("rpc.Add", Numbers{1, 2, 3}, &res); err != nil {
		panic(err)
	}
	fmt.Println("Sum is", res)
	// Output:
	// Sum is 6
}
