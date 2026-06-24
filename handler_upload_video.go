package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)


func getGreatestCommonDivisor(a, b int) int {
	/*
	 *	  https://www.geeksforgeeks.org/dsa/steins-algorithm-for-finding-gcd/
	 *	  Stein's algorithm or binary GCD algorithm method uses binary operations (shifting and comparison) and is efficient for computers. In this algorithm, we can use the following steps to find GCD:
	 *
	 *	  If both numbers are 0, the GCD is 0 (i.e., GCD(0, 0) = 0).                                        *
	 *	  If one number is 0, the GCD is the non-zero number (i.e., GCD(a, 0) = a.
	 *	  If both numbers are even, divide both numbers by 2: GCD(a, b) = 2 × GCD(a/2, b/2)
	 *	  If one number is even and the other is odd, divide the even number by 2: GCD(a, b) = GCD(a/2, b)(or vice versa)
	 *	  If both numbers are odd, subtract the smaller number from the larger one. This reduces the problem to smaller numbers: GCD(a, b) = GCD(∣a − b∣, min⁡(a, b))
	 *	  Repeat steps 3–5 until both numbers match or one of the numbers becomes 0. The non-zero number, multiplied by 2 from the first step, is the GCD.
	 *
	 *	  Let's consider an example for better understanding.
	 *	  Step 1: Both 18 and 24 are even, so divide both by 2 and multiply the result by 2: GCD(18, 24) = 2 × GCD(18/2, 24/2) = 2 × GCD(9, 12)
	 *	  Step 2: 9 is odd and 12 is even, so divide 12 by 2: GCD(9, 12) = GCD(9, 12/2) = GCD(9, 6)
	 *	  Step 3: 9 is odd and 6 is even, so divide 6 by 2: GCD(9, 6) = GCD(9, 6/2) = GCD(9, 3)
	 *	  Step 4: Both 9 and 3 are odd, so subtract the smaller from the larger: GCD(9, 3) = GCD(9 − 3, 3) = GCD(6, 3)
	 *	  Step 5: 6 is even, so divide by 2: GCD(6, 3) = GCD(6/2, 3) = GCD(3, 3)
	 *	  Step 6: Both numbers are equal (3), so the GCD is 3.
	 *	  Final Step: Multiply back the factor of 2 from Step 1: gcd⁡(18, 24) = 2 × 3 = 6
	 *	  Thus, GCD(18, 24) = 6.
	 */

	// GCD(0, b) == b; GCD(a, 0) == a, GCD(0, 0) == 0
	if a == 0 {
		return b
	}

	if b == 0 {
		return a
	}

	// Finding k, where k is the greatest power of 2 that divides both a and b.
	k := 0

	for ((a | b) & 1) == 0 {
		a = a >> 1
		b = b >> 1
		k += 1
	}

	// Dividing a by 2 until a becomes odd
	for (a & 1) == 0 {
		a = a >> 1
	}

	// From here on, 'a' is always odd
	for b != 0 {
		// If b is even, remove all factor of 2 in b
		for (b & 1) == 0 {
			b = b >> 1
		}

		// Now a and b are both odd. Swap if necessary so a <= b, then set b = b - a (which is even)
		if a > b {
			// Swap a and b
			temp := a
			a = b
			b = temp
		}

		b -= a
	}

	// Restore common factors of 2
	return a << k
}

func getVideoAspectRatio(filePath string) (string, error) {
	if len(filePath) < 1 {
		return "", errors.New("Please provide a file path")
	}

	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)

	var jsonData bytes.Buffer
	cmd.Stdout = &jsonData

	if err := cmd.Run(); err != nil {
		return "", errors.New(fmt.Sprintf("Error executing ffprobe command: %v", err))
	}

	var ffProbe struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(jsonData.Bytes(), &ffProbe); err != nil {
		return "", errors.New(fmt.Sprintf("Error unmarshalling ffprobe data: %v", err))
	}
	if len(ffProbe.Streams) < 1 {
		return "", errors.New("no video streams found")
	}

	videoWidth := ffProbe.Streams[0].Width;
	videoHeight := ffProbe.Streams[0].Height;
	//gcd := getGreatestCommonDivisor(videoWidth, videoHeight)
	//aspectRatioWidth := videoWidth / gcd
	//aspectRatioHeight := videoHeight / gcd

	//if aspectRatioWidth == 16 && aspectRatioHeight == 9 {
	if videoWidth == 16 * videoHeight / 9 {
		return "16:9", nil
		//} else if aspectRatioWidth == 9 && aspectRatioHeight == 16 {
	} else if videoHeight == 16 * videoWidth / 9 {

		return "9:16", nil
	}

	return "other", nil
}

func processVideoForFastStart(filePath string) (string, error) {
	if len(filePath) < 1 {
		return "", errors.New("Please provide a file path")
	}

	outputFilePath := filePath + ".processing"

	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputFilePath)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", errors.New(fmt.Sprintf("Error executing ffmpeg command: %s, %v", stderr.String(), err))
	}

	fileInfo, err := os.Stat(outputFilePath)
	if err != nil {
		return "", errors.New(fmt.Sprintf("Error getting stat from processed file: %v", err))
	}
	if fileInfo.Size() == 0 {
		return "", errors.New("Processed file is empty")
	}

	return outputFilePath, nil
}

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s3Client)
	presignedObject, err := presignClient.PresignGetObject(
		context.Background(),
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		},
		s3.WithPresignExpires(expireTime),
	)
	if err != nil {
		return "", errors.New(fmt.Sprintf("Error getting presigned object: %v", err))
	}

	presignedURL := presignedObject.URL
	if len(presignedURL) < 1 {
		return "", errors.New("No URL found")
	}

	return presignedURL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	if video.VideoURL == nil {
		return video, nil
	}

	splitVideoURL := strings.Split(*video.VideoURL, ",")
	if len(splitVideoURL) < 2 {
		return video, nil
	}
	bucket := splitVideoURL[0]
	key := splitVideoURL[1]

	presignedURL, err := generatePresignedURL(cfg.s3Client, bucket, key, 5 * time.Minute)
	if err != nil {
		return video, err
	}

	video.VideoURL = &presignedURL
	return video, nil
}

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
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

	const maxMemory = 1 << 30  // 1 GB
	r.Body = http.MaxBytesReader(w, r.Body, maxMemory)

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Invalid Content-Type", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid video type. Only mp4 files are alllowed", nil)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting video", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized user", nil)
		return
	}

	rando := make([]byte, 32)
	rand.Read(rando)
	baseFileName := base64.RawURLEncoding.EncodeToString(rando)
	fileName := fmt.Sprintf("%s.mp4", baseFileName)

	tempFile, err := os.CreateTemp("", fileName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating video file", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error writing image data to video file", err)
		return
	}

	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error resetting file pointer to beginning of video file", err)
		return
	}

	aspectRatio, err := getVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting aspect ratio from video file", err)
		return
	}

	if aspectRatio == "16:9" {
		fileName = "landscape/" + fileName
	} else if aspectRatio == "9:16" {
		fileName = "portrait/" + fileName
	} else {
		fileName = "other/" + fileName
	}

	fastStartFileName, err := processVideoForFastStart(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error processing video file for fast start", err)
		return
	}
	defer os.Remove(fastStartFileName)

	fastStartFile, err := os.Open(fastStartFileName)
	if err  != nil {
		respondWithError(w, http.StatusInternalServerError, "Error opening fast start video file", err)
		return
	}
	defer fastStartFile.Close()

	if _, err := cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(fileName),
		Body:        fastStartFile,
		ContentType: aws.String(mediaType),
	}); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error uploading file to S3", err)
		return
	}

	videoURL := fmt.Sprintf("%s,%s", cfg.s3Bucket, fileName)
	video.VideoURL = &videoURL

	if err := cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating video", err)
		return
	}

	signedVideo, err := cfg.dbVideoToSignedVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating signed video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, signedVideo)
}
