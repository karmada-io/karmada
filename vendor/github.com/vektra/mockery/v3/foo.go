package main

type baz string

type foo interface {
	Bar() baz
}
