package broadcast

// Size returns the rough size of a BlockReference in bytes.
func (b *BlockReference) Size() int {
	return len(b.Hash) + 100
}

// Size returns the rough size of the TransactionData in bytes.
func (t *TransactionData) Size() int {
	return len(t.Data) + 100
}
