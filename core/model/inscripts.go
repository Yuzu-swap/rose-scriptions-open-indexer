package model

type Inscription struct {
	Hash        string
	Number      uint64 `gorm:"index:idx_number,unique"`
	From        string `gorm:"index:idx_from"`
	To          string `gorm:"index:idx_to"`
	Block       uint64 `gorm:"index:idx_blk"`
	Idx         uint32
	Timestamp   uint64
	ContentType string
	Content     string
}
