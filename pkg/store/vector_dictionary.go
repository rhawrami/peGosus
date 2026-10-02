package store

import (
	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
)

var emptyVectorString dtype.String

// MakeDictionaryStringVectorFromOwnedSegments takes ownership of a flat,
// non-null string dictionary, row indices, and optional row validity. Dictionary
// entries need not be unique. Invalid inputs are released.
func MakeDictionaryStringVectorFromOwnedSegments(dictionary Vector, length int, indices *mem.Segment, validity *BitMap) Vector {
	valid := dictionary.Kind() == VectorFlat && dictionary.TypeID() == dtype.STRT && dictionary.NiN() == 0 && length >= 0 && indices != nil && indices.Len()%4 == 0 && length <= indices.Len()/4 && (validity == nil || validity.Len() == length)
	if valid {
		for row, index := range indices.AsU32T()[:length] {
			if validity != nil && !validity.IsSet(row) {
				continue
			}
			if uint64(index) >= uint64(dictionary.Len()) {
				valid = false
				break
			}
		}
	}
	if !valid {
		dictionary.Release()
		releaseVectorSegments(indices, validity, nil)
		return Vector{}
	}
	return Vector{kind: VectorDictionaryString, dType: dtype.StringT(), length: length, data: indices, validity: validity, dictionary: &dictionary}
}

// Dictionary returns the borrowed flat string dictionary, or nil for a flat vector.
func (v *Vector) Dictionary() *Vector { return v.dictionary }

// DictionaryIndices returns borrowed row IDs, or nil for a flat vector. Invalid
// rows have unspecified IDs. Mutation requires sole ownership; valid rows must
// continue to reference entries within the dictionary.
func (v *Vector) DictionaryIndices() []uint32 {
	if v.kind != VectorDictionaryString {
		return nil
	}
	return v.data.AsU32T()[:v.length]
}

// StringAt returns an immutable borrowed descriptor from either physical representation.
// Invalid rows, invalid indices, and non-string vectors return an empty descriptor.
func (v *Vector) StringAt(row int) *dtype.String {
	if v == nil || v.TypeID() != dtype.STRT || row < 0 || row >= v.length || v.validity != nil && !v.validity.IsSet(row) {
		return &emptyVectorString
	}
	if v.dictionary != nil {
		index := v.data.AsU32T()[row]
		if uint64(index) >= uint64(v.dictionary.Len()) {
			return &emptyVectorString
		}
		return &v.dictionary.Strings()[index]
	}
	return &v.Strings()[row]
}
