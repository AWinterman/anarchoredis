// Copyright 2025 Outreach Corporation. All Rights Reserved.

// Description:

// Package kind:
package kind

type Kind byte
type Category int

const (
	EOL = "\r\n"

	SimpleString   Kind = '+'
	Error          Kind = '-'
	Int            Kind = ':'
	BulkString     Kind = '$'
	Array          Kind = '*'
	Null           Kind = '_'
	Bool           Kind = '#'
	Double         Kind = ','
	BigNumber      Kind = '('
	BulkError      Kind = '!'
	VerbatimString Kind = '='
	Map            Kind = '%'
	Attribute      Kind = '`'
	Set            Kind = '~'
	Push           Kind = '>'
)

const (
	CategorySimple int = iota
	CategoryAggregate
)

func (i Kind) Category() int {
	switch i {
	case SimpleString, Error, Int, Null, Bool, Double, BigNumber:
		return CategorySimple
	case Array, Set, Push, BulkString, BulkError, VerbatimString, Map, Attribute:
		return CategoryAggregate
	default:
		return -1
	}
}

func (i Kind) String() string {
	return Humanize(byte(i))
}

// Humanize returns a human-readable string for the indicator
func Humanize(indicator byte) string {
	switch Kind(indicator) {
	case SimpleString:
		return "SimpleString"
	case Error:
		return "Error"
	case Int:
		return "Int"
	case BulkString:
		return "Bulk"
	case Array:
		return "Seq"
	case Null:
		return "Null"
	case Bool:
		return "Bool"
	case Double:
		return "Double"
	case BigNumber:
		return "BigNumber"
	case VerbatimString:
		return "VerbatimString"
	case Map:
		return "Assoc"
	case Attribute:
		return "Attribute"
	case Set:
		return "Set"
	case Push:
		return "Push"
	default:
		return "Unknown"
	}
}
