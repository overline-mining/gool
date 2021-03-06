package messages

const (
	HANDSHAKE      = "0000R01"
	GET_BLOCKS     = "0006R01"
	BLOCKS         = "0007W01"
	GET_BLOCK      = "0008R01"
	BLOCK          = "0008W01"
	GET_MULTIVERSE = "0009R01"
	MULTIVERSE     = "0010W01"
	GET_SOLUTION   = "0011W01"
	SOLUTION       = "0012W01"
	GET_TXS        = "0013R01"
	TX             = "0014W01"
	TXS            = "0015W01"
	GET_HEADER     = "0016R01"
	HEADER         = "0017W01"
	GET_HEADERS    = "0018R01"
	HEADERS        = "0019W01"
	GET_DATA       = "0020R01"
	DATA           = "0021W01"
	GET_DISTANCE   = "0022R01"
	DISTANCE       = "0023W01"
	PUT_CONFIG     = "0024W01" // OTA updates
	GET_CONFIG     = "0025R01"
	CONFIG         = "0026W01"
	GET_RECORD     = "0027R01"
	RECORD         = "0028W01"
	SEPARATOR      = "[*]"
)
