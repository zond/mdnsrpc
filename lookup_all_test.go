package mdnsrpc

import "fmt"

type Generator int

func (self Generator) Generate(unused struct{}, result *int) (err error) {
	*result = int(self)
	return
}

func ExampleLookupAll() {
	gen1 := Generator(1)
	if _, err := Publish("Generator", gen1); err != nil {
		panic(err)
	}
	gen2 := Generator(2)
	if _, err := Publish("Generator", gen2); err != nil {
		panic(err)
	}
	gen3 := Generator(3)
	if _, err := Publish("Generator", gen3); err != nil {
		panic(err)
	}
	generators, err := LookupAll("Generator")
	if err != nil {
		panic(err)
	}
	res := 0
	for _, client := range generators {
		n := 0
		if err := client.Call("Generate", struct{}{}, &n); err != nil {
			panic(err)
		}
		res += n
	}
	fmt.Println("Sum is", res)
	// Output:
	// Sum is 6
}
