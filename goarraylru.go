package goarraylru

type LRU struct {
	Collisions uint
	Buckets    uint
	Size       uint
	Wrap       bool
	Cache      []*Node
	Hash       func(uint) uint
	Evict      func(index uint, value interface{})
	Evictable  bool
}

type LRUOpts struct {
	Collisions    *uint
	BucketSize    *uint
	IndexedValues *bool
	Evict         *func(index uint, value interface{})
}

type Node struct {
	Index *uint
	Value interface{}
}

func (lru *LRU) Init(max uint, opts LRUOpts) {
	if opts.Collisions != nil {
		lru.Collisions = FactorOfTwo(*opts.Collisions)
	} else if opts.BucketSize != nil {
		lru.Collisions = FactorOfTwo(*opts.BucketSize)
	} else {
		lru.Collisions = FactorOfTwo(4)
	}

	lru.Buckets = FactorOf(max, lru.Collisions) / lru.Collisions

	// Using 16bit hasing to bucket. As such the index must be <0xffff(65536).
	for lru.Buckets > 65536 {
		lru.Buckets >>= 1
		lru.Collisions <<= 1
	}

	lru.Size = lru.Buckets * lru.Collisions
	// If Indexed values doesn't exist default to false.
	if opts.IndexedValues != nil {
		lru.Wrap = !*opts.IndexedValues
	} else {
		lru.Wrap = true
	}

	lru.Cache = make([]*Node, lru.Size)

	// The LRU Hash member is a function.
	if lru.Buckets == 65536 {
		lru.Hash = Crc16
	} else {
		lru.Hash = MaskedHash(lru.Buckets - 1)
	}

	if opts.Evict != nil {
		lru.Evict = *opts.Evict
		lru.Evictable = true
	} else {
		lru.Evictable = false
	}
}

func (lru *LRU) Set(index uint, val interface{}) {
	var pageStart, pageEnd, ptr uint
	pageStart = lru.Collisions * lru.Hash(index)
	pageEnd = pageStart + lru.Collisions
	ptr = pageStart
	var page *Node
	page = nil

	for ptr < pageEnd {
		page = lru.Cache[ptr]

		if page == nil {
			// There is no existing version so store the new data.
			if lru.Wrap {
				page = &Node{&index, val}
			} else {
				page = &Node{nil, val}
			}
			Move(lru.Cache, pageStart, ptr, page)
			return
		}

		if page.Index != nil {
			if *page.Index == index {
				page.Value = val
				Move(lru.Cache, pageStart, ptr, page)
				return
			}
		}

		ptr++
	}

	// In this case the bucket is full so update the oldest element.
	if lru.Wrap {
		if lru.Evict != nil {
			lru.Evict(*page.Index, page.Value)
		}
		page.Index = &index
		page.Value = val
	} else {
		if lru.Evict != nil {
			lru.Evict(0, page)
		}
		lru.Cache[ptr-1] = &Node{nil, val}
	}
	Move(lru.Cache, pageStart, ptr-1, page)
}

func (lru *LRU) Get(index uint) *Node {
	var pageStart, pageEnd, ptr uint
	pageStart = lru.Collisions * lru.Hash(index)
	pageEnd = pageStart + lru.Collisions
	ptr = pageStart

	for ptr < pageEnd {
		var page *Node
		page = lru.Cache[ptr]
		ptr++

		if page == nil {
			return nil
		}
		if page.Index == nil {
			continue
		}
		if *page.Index != index {
			continue
		}
		Move(lru.Cache, pageStart, ptr-1, page)

		return page
	}
	return nil
}

func Move(list []*Node, index uint, itemIndex uint, item *Node) {
	var indexBefore uint
	for itemIndex > index {
		indexBefore = itemIndex - 1
		list[itemIndex] = list[indexBefore]
	}
	list[index] = item
}

func FactorOf(n, factor uint) uint {
	n = FactorOfTwo(n)
	for n&(factor-1) != 0 {
		n <<= 1
	}
	return n
}

func FactorOfTwo(n uint) uint {
	if n != 0 && (n&(n-1)) == 0 {
		return n
	}
	var p uint
	p = 1
	for p < n {
		p <<= 1
	}
	return p
}

func MaskedHash(mask uint) func(uint) uint {
	return func(n uint) uint {
		return Crc16(n) & mask
	}
}
