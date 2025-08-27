package services

import "testing"

func BenchmarkMarshalTags(b *testing.B) {
	b.ReportAllocs()
	b.Run("with tag", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := marshalTags("team"); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("without tag", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := marshalTags(""); err != nil {
				b.Fatal(err)
			}
		}
	})
}
