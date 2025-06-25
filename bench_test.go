package main

import (
	"testing"
	"time"
)

func BenchmarkServiceMethod(b *testing.B) {
	// Setup code here (if needed)

	for i := 0; i < b.N; i++ {
		// Call the method you want to benchmark
		time.Sleep(10 * time.Millisecond) // Simulating a service method call
	}
}

func BenchmarkHandlerMethod(b *testing.B) {
	// Setup code here (if needed)

	for i := 0; i < b.N; i++ {
		// Simulate a handler method call
		time.Sleep(5 * time.Millisecond) // Simulating a handler method call
	}
}