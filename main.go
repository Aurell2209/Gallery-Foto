package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Photo struct untuk model data
type Photo struct {
	ID        uint   `gorm:"primaryKey"`
	Judul     string `gorm:"not null"` // judul juga tidak boleh kosong
	Deskripsi string
	Gambar    string         `gorm:"not null"` // gambar tidak boleh kosong
	CreatedAt time.Time      // waktu saat dibuat
	UpdatedAt time.Time      // waktu saat di perbarui
	DeletedAt gorm.DeletedAt `gorm:"index"` // Untuk hapus
}

// Data untuk dikirim ke template HTML
type HomeData struct {
	Photos      []Photo // daftar dari Photo yang akan ditampilkan di tabel galeri
	SearchQuery string  // Tambahkan ini untuk menyimpan query pencarian
}

func connectDB() (*gorm.DB, error) {
	dsn := "root:@tcp(127.0.0.1:3306)/gallery?charset=utf8mb4&parseTime=True&loc=Local"
	// dsn adalah Data Source Name yang berisi kredensial koneksi ke database
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("gagal terhubung ke database: %w", err)
	}

	// AutoMigrate akan membuat tabel jika belum ada
	err = db.AutoMigrate(&Photo{})
	if err != nil {
		return nil, fmt.Errorf("gagal auto migrate database: %w", err)
	}
	return db, nil
}

// Halaman utama
func homeHandler(w http.ResponseWriter, r *http.Request) {
	db, err := connectDB()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var photos []Photo
	query := db.Order("id asc") // Urutkan berdasarkan ID terkecil ke terbesar

	// Pencarian
	// menambahkan WHERE
	searchQuery := r.URL.Query().Get("q")
	if searchQuery != "" {
		// mencari menggunakan judul / diskripsi
		query = query.Where("judul LIKE ? OR deskripsi LIKE ?", "%"+searchQuery+"%", "%"+searchQuery+"%")
	}
	// mengambil data yang dicari
	result := query.Find(&photos)
	if result.Error != nil {
		http.Error(w, "Gagal mengambil data foto: "+result.Error.Error(), http.StatusInternalServerError)
		return
	}

	data := HomeData{
		Photos:      photos,
		SearchQuery: searchQuery, // Kirimkan kembali query pencarian ke template
	}

	tmpl := template.Must(template.ParseFiles("template/index.html"))
	tmpl.Execute(w, data)
}

// Menambahkan foto
func addPhotoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	db, err := connectDB()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	judul := r.FormValue("Judul")
	deskripsi := r.FormValue("Deskripsi")

	file, handler, err := r.FormFile("Gambar")
	if err != nil {
		http.Error(w, "Gagal mendapatkan file gambar: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Batasi ukuran file (2MB)
	if handler.Size > 2*1024*1024 { // 2MB
		http.Error(w, "Ukuran file terlalu besar. Maksimal 2MB.", http.StatusBadRequest)
		return
	}

	// Tipe file
	fileExt := filepath.Ext(handler.Filename)
	allowedExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
	}
	if !allowedExts[fileExt] {
		http.Error(w, "Tipe file tidak diizinkan. Hanya JPG, JPEG, PNG.", http.StatusBadRequest)
		return
	}

	// Buat nama gambar
	fileName := fmt.Sprintf("%d%s", time.Now().UnixNano(), fileExt)
	filePath := filepath.Join("uploads", fileName)

	dst, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Gagal menyimpan file gambar: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Gagal menyalin file gambar: "+err.Error(), http.StatusInternalServerError)
		return
	}

	photo := Photo{
		Judul:     judul,
		Deskripsi: deskripsi,
		Gambar:    fileName, // menampilkan nama file/gambar
	}

	result := db.Create(&photo)
	if result.Error != nil {
		http.Error(w, "Gagal menyimpan data foto ke database: "+result.Error.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Edit foto
func editPhotoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	db, err := connectDB()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	idStr := r.FormValue("ID")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "ID foto tidak valid", http.StatusBadRequest)
		return
	}

	judul := r.FormValue("Judul")
	deskripsi := r.FormValue("Deskripsi")

	var photo Photo
	result := db.First(&photo, id)
	if result.Error != nil {
		http.Error(w, "Foto tidak ditemukan: "+result.Error.Error(), http.StatusNotFound)
		return
	}

	// Update data foto
	photo.Judul = judul
	photo.Deskripsi = deskripsi

	result = db.Save(&photo)
	if result.Error != nil {
		http.Error(w, "Gagal mengupdate data foto: "+result.Error.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Mengapus foto
func deletePhotoHandler(w http.ResponseWriter, r *http.Request) {
	db, err := connectDB()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "ID foto tidak valid", http.StatusBadRequest)
		return
	}

	var photo Photo
	result := db.First(&photo, id)
	if result.Error != nil {
		http.Error(w, "Foto tidak ditemukan: "+result.Error.Error(), http.StatusNotFound)
		return
	}

	// Hapus file gambar dari server
	imagePath := filepath.Join("uploads", photo.Gambar)
	if err := os.Remove(imagePath); err != nil {
		log.Printf("Gagal menghapus file gambar %s: %v", imagePath, err)
	}

	// Hapus data dari database (hard delete)
	result = db.Delete(&photo, id)
	if result.Error != nil {
		http.Error(w, "Gagal menghapus foto dari database: "+result.Error.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func viewPhotoHandler(w http.ResponseWriter, r *http.Request) {
	photoIDStr := r.URL.Query().Get("id") // Mendapatkan ID dari query parameter, misal /photos/view?id=123
	if photoIDStr == "" {
		http.Error(w, "ID foto tidak diberikan", http.StatusBadRequest)
		return
	}

	photoID, err := strconv.ParseUint(photoIDStr, 10, 32)
	if err != nil {
		http.Error(w, "ID foto tidak valid", http.StatusBadRequest)
		return
	}

	db, err := connectDB()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var photo Photo
	result := db.First(&photo, photoID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Foto tidak ditemukan", http.StatusNotFound)
		} else {
			http.Error(w, "Gagal mengambil data foto: "+result.Error.Error(), http.StatusInternalServerError)
		}
		return
	}

	tmpl := template.Must(template.ParseFiles("template/view_photo.html"))
	tmpl.Execute(w, photo)
}

func main() {
	// Pastikan folder 'uploads' ada
	err := os.MkdirAll("uploads", os.ModePerm)
	if err != nil {
		log.Fatalf("Gagal membuat direktori uploads: %v", err)
	}

	// Koneksi database saat aplikasi dimulai
	_, err = connectDB()
	if err != nil {
		log.Fatalf("Gagal terhubung ke database: %v", err)
	}

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/photos", homeHandler) // Endpoint tambahan untuk /photos
	http.HandleFunc("/tambah", addPhotoHandler)
	http.HandleFunc("/edit", editPhotoHandler)
	http.HandleFunc("/hapus", deletePhotoHandler)
	http.HandleFunc("/photos/view", viewPhotoHandler) // Endpoint untuk melihat detail foto

	// Serving static files dari direktori 'uploads'
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

	log.Println("Server berjalan di http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
