package domain

import "testing"

// samples là 100 test case chuẩn hóa đầu-cuối của tài liệu 0.1 mục 19: tên vào và lượng
// tồn canonical mong đợi. groupName đối chiếu với taxonomy 0.2 ở normalization_test.go.
var samples = []struct {
	n          int
	input      string
	groupName  string
	detailName string
	qty        float64
	unit       Unit
}{
	{1, "Vinamilk không đường 1L", "Sữa tươi", "Vinamilk Không đường", 1000, "ml"},
	{2, "TH True Milk ít đường 4×180ml", "Sữa tươi", "TH True Milk Ít đường", 720, "ml"},
	{3, "Dalat Milk 950ml", "Sữa tươi", "Dalat Milk Không đường", 950, "ml"},
	{4, "Fami nguyên chất 1L", "Sữa đậu nành", "Fami Nguyên chất", 1000, "ml"},
	{5, "Vinamilk Sure Diacerna 2×900g", "Sữa bột", "Vinamilk Sure Diacerna", 1800, "g"},
	{6, "Vinamilk Sure Prevent 2×900g", "Sữa bột", "Vinamilk Sure Prevent", 1800, "g"},
	{7, "Ensure Gold 850g", "Sữa bột", "Ensure Gold", 850, "g"},
	{8, "Anlene Gold 800g", "Sữa bột", "Anlene Gold", 800, "g"},
	{9, "Lavie 6×500ml", "Nước uống", "Lavie Nước khoáng", 3000, "ml"},
	{10, "Coca-Cola 1.5L", "Nước ngọt", "Coca-Cola Original", 1500, "ml"},
	{11, "Pepsi 1.5L", "Nước ngọt", "Pepsi Original", 1500, "ml"},
	{12, "Twister cam 1L", "Nước trái cây", "Twister Cam", 1000, "ml"},
	{13, "Simply dầu đậu nành 2L", "Dầu ăn", "Simply Đậu nành", 2000, "ml"},
	{14, "Simply dầu gạo 1L", "Dầu ăn", "Simply Dầu gạo", 1000, "ml"},
	{15, "Neptune 1L", "Dầu ăn", "Neptune", 1000, "ml"},
	{16, "Nam Ngư 750ml", "Nước mắm", "Nam Ngư", 750, "ml"},
	{17, "Chin-su 500ml", "Nước mắm", "Chin-su", 500, "ml"},
	{18, "Maggi nước tương 700ml", "Nước tương", "Maggi", 700, "ml"},
	{19, "Tương ớt Chin-su 500g", "Tương ớt", "Chin-su", 500, "g"},
	{20, "Giấm gạo 500ml", "Giấm", "Giấm gạo", 500, "ml"},
	{21, "Gạo ST25 5kg", "Gạo", "ST25", 5000, "g"},
	{22, "Gạo Jasmine 5kg", "Gạo", "Jasmine", 5000, "g"},
	{23, "Yến mạch Quaker 600g", "Yến mạch", "Quaker Cán dẹt", 600, "g"},
	{24, "Bột mì Meizan 1kg", "Bột mì", "Meizan Đa dụng", 1000, "g"},
	{25, "Đường Biên Hòa 1kg", "Đường", "Biên Hòa Đường trắng", 1000, "g"},
	{26, "Muối i-ốt 500g", "Muối", "Muối i-ốt", 500, "g"},
	{27, "Ba rọi heo 500g", "Thịt heo", "Ba rọi", 500, "g"},
	{28, "Thịt heo xay 400g", "Thịt heo", "Thịt xay", 400, "g"},
	{29, "Ức gà CP 500g", "Thịt gà", "CP Ức gà", 500, "g"},
	{30, "Đùi gà 1kg", "Thịt gà", "Đùi gà", 1000, "g"},
	{31, "Cá hồi 300g", "Cá hồi", "Cá hồi phi lê", 300, "g"},
	{32, "Cá basa 500g", "Cá basa", "Cá basa phi lê", 500, "g"},
	{33, "Tôm thẻ 500g", "Tôm", "Tôm thẻ", 500, "g"},
	{34, "Mực ống 500g", "Mực", "Mực ống", 500, "g"},
	{35, "Trứng Ba Huân 10 quả", "Trứng", "Ba Huân Trứng gà", 10, "item"},
	{36, "Trứng vịt 10 quả", "Trứng", "Trứng vịt", 10, "item"},
	{37, "Trứng gà ta 6 quả", "Trứng", "Trứng gà ta", 6, "item"},
	{38, "Táo Fuji 1.2kg", "Táo", "Fuji", 1200, "g"},
	{39, "Táo Envy 800g", "Táo", "Envy", 800, "g"},
	{40, "Cam vàng 1.5kg", "Cam", "Cam vàng", 1500, "g"},
	{41, "Chuối 1.2kg", "Chuối", "Chuối", 1200, "g"},
	{42, "Dưa hấu 3.4kg", "Dưa hấu", "Không hạt", 3400, "g"},
	{43, "Cà chua 500g", "Cà chua", "Cà chua", 500, "g"},
	{44, "Cà rốt 1kg", "Cà rốt", "Cà rốt", 1000, "g"},
	{45, "Rau cải 400g", "Rau cải", "Cải xanh", 400, "g"},
	{46, "Rau muống 500g", "Rau muống", "Rau muống", 500, "g"},
	{47, "Khoai tây 1kg", "Khoai tây", "Khoai tây", 1000, "g"},
	{48, "Sandwich trắng 500g", "Bánh mì", "Sandwich trắng", 500, "g"},
	{49, "Bánh mì nguyên cám 450g", "Bánh mì", "Nguyên cám", 450, "g"},
	{50, "Baguette 2 ổ", "Bánh mì", "Baguette", 2, "item"},
	{51, "Hảo Hảo 5×75g", "Mì ăn liền", "Hảo Hảo Tôm chua cay", 375, "g"},
	{52, "Vifon phở bò 5×65g", "Phở ăn liền", "Vifon Phở bò", 325, "g"},
	{53, "Cháo Gấu Đỏ 5×50g", "Cháo ăn liền", "Gấu Đỏ Thịt bằm", 250, "g"},
	{54, "Bún Safoco 500g", "Bún khô", "Safoco Bún gạo", 500, "g"},
	{55, "Xúc xích Vissan 175g", "Xúc xích", "Vissan Tiệt trùng", 175, "g"},
	{56, "Romano Classic 650ml", "Dầu gội nam", "Romano Classic", 650, "ml"},
	{57, "Romano Force 900ml", "Dầu gội nam", "Romano Force", 900, "ml"},
	{58, "Clear Men 630ml", "Dầu gội nam", "Clear Men Cool Sport", 630, "ml"},
	{59, "Sunsilk 650ml", "Dầu gội", "Sunsilk Óng mượt", 650, "ml"},
	{60, "Dove dầu xả 620ml", "Dầu xả", "Dove Phục hồi", 620, "ml"},
	{61, "Dove sữa tắm 900ml", "Sữa tắm", "Dove Dưỡng ẩm", 900, "ml"},
	{62, "Lifebuoy sữa tắm 800g", "Sữa tắm", "Lifebuoy Matcha", 800, "g"},
	{63, "Listerine 750ml", "Nước súc miệng", "Listerine Cool Mint", 750, "ml"},
	{64, "P/S 180g", "Kem đánh răng", "P/S Bảo vệ 123", 180, "g"},
	{65, "Lifebuoy gel rửa tay 500ml", "Nước rửa tay", "Lifebuoy Gel", 500, "ml"},
	{66, "Ariel nước giặt 3.2L", "Nước giặt", "Ariel Cửa trước", 3200, "ml"},
	{67, "Attack nước giặt 2.4L", "Nước giặt", "Attack Hương hoa", 2400, "ml"},
	{68, "OMO nước giặt 3.6kg", "Nước giặt", "OMO Matic Comfort", 3600, "g"},
	{69, "OMO bột giặt 3kg", "Bột giặt", "OMO", 3000, "g"},
	{70, "Comfort 3L", "Nước xả", "Comfort", 3000, "ml"},
	{71, "Downy 2.3L", "Nước xả", "Downy", 2300, "ml"},
	{72, "Sunlight 750ml", "Nước rửa chén", "Sunlight Chanh", 750, "ml"},
	{73, "Sunlight lau sàn 1L", "Nước lau sàn", "Sunlight", 1000, "ml"},
	{74, "Vim toilet 880ml", "Nước tẩy toilet", "Vim", 880, "ml"},
	{75, "Gift lau kính 580ml", "Nước lau kính", "Gift", 580, "ml"},
	{76, "Pulppy 10 cuộn", "Giấy vệ sinh", "Pulppy 3 lớp", 10, "item"},
	{77, "Bless You 12 cuộn", "Giấy vệ sinh", "Bless You", 12, "item"},
	{78, "Kleenex 180 tờ", "Khăn giấy", "Kleenex Hộp", 180, "item"},
	{79, "Pulppy 3×180 tờ", "Khăn giấy", "Pulppy Hộp", 540, "item"},
	{80, "Bobby khăn ướt 100 miếng", "Khăn ướt", "Bobby", 100, "item"},
	{81, "Túi rác 3×20 túi", "Túi rác", "Túi rác tự hủy", 60, "item"},
	{82, "Scotch-Brite 3 miếng", "Miếng rửa chén", "Scotch-Brite", 3, "item"},
	{83, "Bioderma 500ml", "Nước tẩy trang", "Bioderma Sensibio H2O", 500, "ml"},
	{84, "Cetaphil 500ml", "Sữa rửa mặt", "Cetaphil Gentle Cleanser", 500, "ml"},
	{85, "Klairs toner 180ml", "Toner", "Klairs Unscented", 180, "ml"},
	{86, "The Ordinary 30ml", "Serum", "The Ordinary Niacinamide 10%", 30, "ml"},
	{87, "La Roche-Posay B5 30ml", "Serum", "La Roche-Posay Hyalu B5", 30, "ml"},
	{88, "CeraVe Cream 50ml", "Kem dưỡng", "CeraVe Moisturising Cream", 50, "ml"},
	{89, "Nivea Creme 60g", "Kem dưỡng", "Nivea Creme", 60, "g"},
	{90, "Anessa 60ml", "Kem chống nắng", "Anessa Perfect UV", 60, "ml"},
	{91, "Vaseline Lip Therapy 20g", "Son dưỡng", "Vaseline Lip Therapy", 20, "g"},
	{92, "Mediheal 10 miếng", "Mặt nạ", "Mediheal Tea Tree", 10, "item"},
	{93, "Bobby size L 68 miếng", "Tã quần", "Bobby Size L", 68, "item"},
	{94, "Huggies XL 62 miếng", "Tã quần", "Huggies Dry Size XL", 62, "item"},
	{95, "Merries M 64 miếng", "Tã dán", "Merries Size M", 64, "item"},
	{96, "Finish 30 viên", "Viên rửa chén", "Finish All in One", 30, "item"},
	{97, "Ariel 3in1 30 viên", "Viên giặt", "Ariel 3in1", 30, "item"},
	{98, "Energizer AA 4 viên", "Pin AA", "Energizer AA", 4, "item"},
	{99, "Colgate SlimSoft 3 cái", "Bàn chải đánh răng", "Colgate SlimSoft", 3, "item"},
	{100, "Blackmores Vitamin C 150 viên", "Vitamin C", "Blackmores 1000mg", 150, "item"},
}

func TestParsePackagingSamples(t *testing.T) {
	for _, c := range samples {
		p := ParsePackaging(c.input)
		if !p.Known() || p.Total != c.qty || p.Unit != c.unit {
			t.Errorf("#%d %q: got %v %s (count %v size %v %s), want %v %s", c.n, c.input, p.Total, p.Unit, p.Count, p.Size, p.SizeUnit, c.qty, c.unit)
		}
	}
}

func TestParsePackagingAC(t *testing.T) {
	cases := []struct {
		in   string
		qty  float64
		unit Unit
	}{
		{"Combo 2 lon Sữa Bột Vinamilk Sure Diacerna 900g - 37 dưỡng chất với công thức có chỉ số đường huyết thấp", 1800, UnitGram}, // AC-01
		{"2 combo × 3 hộp × 100 ml", 600, UnitML}, // AC-02
		{"Dầu ăn 4L", 4000, UnitML},               // AC-03
		{"Gạo 5kg", 5000, UnitGram},               // AC-04
		{"Sữa tắm 800g", 800, UnitGram},           // AC-05: giữ g, không đổi sang ml
		{"3 lốc × 4 hộp × 180 ml", 2160, UnitML},
		{"2 túi × 2.5 kg", 5000, UnitGram},
		{"2 vỉ × 6 quả", 12, UnitItem},
		{"3 hộp × 10 viên", 30, UnitItem},
		{"Vitamin C 1000mg 30 viên", 30, UnitItem}, // mg là hàm lượng, không phải quy cách
		{"Gạo ST25 5kg", 5000, UnitGram},
		{"Ariel 3in1 30 viên", 30, UnitItem},
		{"VINAMILK SUA BOT SURE DIACERNA 2X900G", 1800, UnitGram},
		{"TH TRUE MILK IT DUONG 4x180ML", 720, UnitML},
	}
	for _, c := range cases {
		p := ParsePackaging(c.in)
		if p.Total != c.qty || p.Unit != c.unit {
			t.Errorf("%q: got %v %s, want %v %s", c.in, p.Total, p.Unit, c.qty, c.unit)
		}
	}
	if p := ParsePackaging("Rau thơm"); p.Known() {
		t.Errorf("no packaging should be unknown, got %+v", p)
	}
	if p := ParsePackaging("Sữa tươi 1L").ForLines(2); p.Total != 2000 || p.Count != 2 {
		t.Errorf("ForLines: %+v", p)
	}
}
