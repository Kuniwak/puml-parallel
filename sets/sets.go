package sets

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

type Set[T comparable] map[T]struct{}

func NewSet[T comparable](items ...T) Set[T] {
	s := make(Set[T])
	for _, item := range items {
		s.Add(item)
	}
	return s
}

func (s Set[T]) Add(item T) {
	s[item] = struct{}{}
}

func (s Set[T]) Remove(item T) {
	delete(s, item)
}

func (s Set[T]) Contains(item T) bool {
	_, ok := s[item]
	return ok
}

func (s Set[T]) Len() int {
	return len(s)
}

func (s Set[T]) IsEmpty() bool {
	return len(s) == 0
}

var (
	openBrace  = []byte{'{'}
	closeBrace = []byte{'}'}
	sep        = []byte{',', ' '}
)

type StringConvertible interface {
	comparable
	String() string
}

func String[T StringConvertible](s Set[T]) string {
	sb := &strings.Builder{}
	if _, err := WriteTo(sb, s); err != nil {
		panic(fmt.Errorf("String[T StringConvertible]: %w", err))
	}
	return sb.String()
}

func WriteTo[T StringConvertible](w io.Writer, s Set[T]) (int64, error) {
	var n int64
	n2, err := w.Write(openBrace)
	n += int64(n2)
	if err != nil {
		return n, err
	}

	ss := make([]string, 0, len(s))
	for item := range s {
		ss = append(ss, item.String())
	}

	slices.Sort(ss)

	n2, err = w.Write(openBrace)
	n += int64(n2)
	if err != nil {
		return n, err
	}

	for i, item := range ss {
		if i > 0 {
			n2, err = w.Write(sep)
			n += int64(n2)
			if err != nil {
				return n, err
			}
		}
		n3, err := io.WriteString(w, item)
		n += int64(n3)
		if err != nil {
			return n, err
		}
	}

	n2, err = w.Write(closeBrace)
	n += int64(n2)
	if err != nil {
		return n, err
	}

	return n, err
}

func IsStrictSubset[T comparable](s1, s2 Set[T]) bool {
	if s1.Len() >= s2.Len() {
		return false
	}
	for item := range s1 {
		if !s2.Contains(item) {
			return false
		}
	}
	return true
}

func IsSubset[T comparable](s1, s2 Set[T]) bool {
	for item := range s1 {
		if !s2.Contains(item) {
			return false
		}
	}
	return true
}

func Union[T comparable](s1, s2 Set[T]) Set[T] {
	result := NewSet[T]()
	for item := range s1 {
		result.Add(item)
	}
	for item := range s2 {
		result.Add(item)
	}
	return result
}

func Intersection[T comparable](s1, s2 Set[T]) Set[T] {
	result := NewSet[T]()
	for item := range s1 {
		if s2.Contains(item) {
			result.Add(item)
		}
	}
	return result
}

func Difference[T comparable](s1, s2 Set[T]) Set[T] {
	result := NewSet[T]()
	for item := range s1 {
		if !s2.Contains(item) {
			result.Add(item)
		}
	}
	return result
}

func SymmetricDifference[T comparable](s1, s2 Set[T]) Set[T] {
	result := NewSet[T]()
	for item := range s1 {
		if !s2.Contains(item) {
			result.Add(item)
		}
	}
	for item := range s2 {
		if !s1.Contains(item) {
			result.Add(item)
		}
	}
	return result
}

func Equal[T comparable](s1, s2 Set[T]) bool {
	if s1.Len() != s2.Len() {
		return false
	}
	for item := range s1 {
		if !s2.Contains(item) {
			return false
		}
	}
	return true
}
