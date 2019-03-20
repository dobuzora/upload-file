package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	uuid "github.com/gofrs/uuid"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

const maxUploadSize = 2 * 1024 * 1024

var uploadPath string

func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatal(fmt.Sprintf("Can not read env file : %v", err))
	}
	uploadPath = os.Getenv("TMP_DIR")
}

func main() {
	fmt.Println(uploadPath)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	r := mux.NewRouter()

	r.Methods("POST").Path("/upload").Handler(appHandler(uploadFileHandler))
	http.Handle("/", handlers.CombinedLoggingHandler(os.Stderr, r))

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))

}

// uploadFileHandler upload a image file.
func uploadFileHandler(w http.ResponseWriter, r *http.Request) *appError {
	// validate file size
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		return appErrorf(err, "File Too Big: %v", err)
	}

	// parse and validate file and post parameters
	file, _, err := r.FormFile("image")
	if err != nil {
		return appErrorf(err, "Not Parse : %v", err)
	}
	defer file.Close()
	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		return appErrorf(err, "Not Read : %v", err)
	}

	// check file type, detectcontenttype only needs the first 512 bytes
	filetype := http.DetectContentType(fileBytes)
	switch filetype {
	case "image/jpeg", "image/jpg":
	case "image/gif", "image/png":
	case "application/pdf":
		break
	default:
		return appErrorf(err, "Not Support this extention", nil)
	}
	fileID := uuid.Must(uuid.NewV4()).String()
	fileEndings, err := mime.ExtensionsByType(filetype)
	log.Println(fileEndings)
	if err != nil {
		return appErrorf(err, "Can not read file type : %v", err)
	}
	newPath := filepath.Join(uploadPath, fileID+fileEndings[0])
	log.Printf("FileType: %s, File: %s\n", filetype, newPath)

	// write file
	newFile, err := os.Create(newPath)
	if err != nil {
		return appErrorf(err, "Can not create file : %v", err)
	}
	defer newFile.Close() // idempotent, okay to call twice
	if _, err := newFile.Write(fileBytes); err != nil || newFile.Close() != nil {
		return appErrorf(err, "Can not ", err)
	}
	w.Write([]byte("SUCCESS"))
	return nil
}

type appHandler func(http.ResponseWriter, *http.Request) *appError

type appError struct {
	Error   error
	Message string
	Code    int
}

func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if e := fn(w, r); e != nil {
		log.Printf("Handler error : status code : %d, message :%s, underlying err : %#v", e.Code, e.Message, e.Error)

		http.Error(w, e.Message, e.Code)
	}
}

func appErrorf(err error, format string, v ...interface{}) *appError {
	return &appError{
		Error:   err,
		Message: fmt.Sprintf(format, v...),
		Code:    500,
	}
}
