package taxonomy

import "github.com/smartlife-lab/mimi/backend/internal/domain"

// brandNames là brand dictionary (7.1 mục 4): cách viết chính thức của brand, ưu tiên hơn
// quy tắc "viết hoa chữ cái đầu field". Tra theo chuỗi đã bỏ dấu, bỏ khoảng trắng và dấu
// câu, nên "COCA COLA", "coca-cola", "CocaCola" đều ra "Coca-Cola".
var brandNames = []string{
	"Vinamilk", "TH true MILK", "Dalat Milk", "Fami", "Ensure", "Anlene", "Abbott", "Nestlé", "Milo", "Nutifood",
	"Lavie", "Aquafina", "Coca-Cola", "Pepsi", "7Up", "Twister", "Sting", "Red Bull", "Number 1",
	"Simply", "Neptune", "Tường An", "Nam Ngư", "Chin-su", "Maggi", "Knorr", "Ajinomoto", "Cholimex",
	"Meizan", "Quaker", "Biên Hòa", "CP", "Ba Huân", "Hảo Hảo", "Omachi", "Vifon", "Acecook", "Gấu Đỏ",
	"Safoco", "Vissan", "Đức Việt", "Romano", "Clear", "Clear Men", "Sunsilk", "Dove", "Lifebuoy", "Head & Shoulders",
	"Pantene", "TRESemmé", "Listerine", "P/S", "Colgate", "Close-Up", "Sensodyne", "Oral-B",
	"Ariel", "OMO", "Tide", "Attack", "Comfort", "Downy", "Surf", "Sunlight", "Vim", "Gift", "Mỹ Hảo",
	"Pulppy", "Bless You", "Kleenex", "Scotch-Brite", "Bioderma", "Cetaphil", "Klairs", "The Ordinary",
	"La Roche-Posay", "CeraVe", "Nivea", "Anessa", "Vaseline", "Mediheal", "L'Oréal", "Garnier", "Hada Labo",
	"Bobby", "Huggies", "Merries", "Pampers", "Moony", "Finish", "Energizer", "Panasonic", "Blackmores",
	"Kotex", "Diana", "Durex", "WinMart", "Co.opmart", "Bách Hóa Xanh", "AEON", "Lotte Mart", "GO!", "MM Mega Market",
	"Orion", "Oreo", "Kinh Đô", "Lay's", "Pringles", "KitKat", "Ferrero Rocher", "Trung Nguyên", "G7", "Nescafé",
	"Highlands Coffee", "Lipton", "Cozy", "Dilmah", "ST25",
}

var brandByKey = func() map[string]string {
	m := make(map[string]string, len(brandNames))
	for _, b := range brandNames {
		m[brandKey(b)] = b
	}
	return m
}()

func brandKey(s string) string {
	var out []rune
	for _, r := range domain.NormalizeVi(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			out = append(out, r)
		}
	}
	return string(out)
}

// Brand trả cách viết chuẩn nếu brand có trong dictionary; dùng làm domain.BrandLookup.
func Brand(name string) (string, bool) {
	b, ok := brandByKey[brandKey(name)]
	return b, ok
}
