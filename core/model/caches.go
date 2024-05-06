package model

type Token struct {
	Tick          string `gorm:"index:idx_tick,unique"`
	Number        uint64
	Precision     int
	Max           *DDecimal
	Limit         *DDecimal
	Minted        *DDecimal
	Progress      int32
	Holders       int32
	Trxs          int32
	CreatedAt     uint64
	CompletedAt   int64
	DeployAddress string
	DeployHash    string
}

type ListedRecord struct {
	Hash        string `gorm:"index:idx_hash,unique"`
	Tick        string `gorm:"index:idx_tick"`
	OriginAddr  string
	ListedTo    string
	TransferdTo string
	Amount      *DDecimal
	ListedTs    uint64
	TransferdTs uint64
}
