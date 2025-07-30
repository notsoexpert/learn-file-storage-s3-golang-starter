package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Bad content type", err)
		return
	}

	var extension string
	if strings.Contains(mediaType, "image/png") {
		extension = "png"
	} else if strings.Contains(mediaType, "image/jpeg") {
		extension = "jpeg"
	} else {
		respondWithError(w, http.StatusBadRequest, "Invalid thumbnail type", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to find matching data", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized request", err)
		return
	}

	var fnBytes []byte = make([]byte, 32)
	_, _ = rand.Read(fnBytes)
	fileName := base64.RawURLEncoding.EncodeToString(fnBytes)

	filePath := filepath.Join(cfg.assetsRoot, fileName+"."+extension)
	osFile, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to store content", err)
		return
	}
	defer osFile.Close()

	_, err = io.Copy(osFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Write failure", err)
		return
	}

	if video.ThumbnailURL == nil {
		video.ThumbnailURL = new(string)
	}
	*video.ThumbnailURL = fmt.Sprintf(`http://localhost:%s/assets/%s.%s`, cfg.port, fileName, extension)

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update database", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
