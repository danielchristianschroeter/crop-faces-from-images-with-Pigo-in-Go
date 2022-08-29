package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"

	"log"
	"os"
	"path/filepath"
	"strconv"

	pigo "github.com/esimov/pigo/core"
)

// DetectionResult contains the coordinates of the detected faces and the base64 converted image.
type DetectionResult struct {
	Faces       []image.Rectangle
	ImageBase64 string
}

// Find all faces in image
func DetectFaces(src *image.NRGBA) ([]pigo.Detection, error) {
	pixels := pigo.RgbToGrayscale(src)
	cols, rows := src.Bounds().Max.X, src.Bounds().Max.Y

	cParams := pigo.CascadeParams{
		MinSize:     20,
		MaxSize:     2000,
		ShiftFactor: 0.1,
		ScaleFactor: 1.1,

		ImageParams: pigo.ImageParams{
			Pixels: pixels,
			Rows:   rows,
			Cols:   cols,
			Dim:    cols,
		},
	}

	cascadeFile, err := os.ReadFile("./facefinder")
	if err != nil {
		return nil, err
	}

	pigo := pigo.NewPigo()
	// Unpack the binary file. This will return the number of cascade trees,
	// the tree depth, the threshold and the prediction from tree's leaf nodes.
	classifier, err := pigo.Unpack(cascadeFile)
	if err != nil {
		return nil, err
	}

	// Run the classifier over the obtained leaf nodes and return the detection results.
	// The result contains quadruplets representing the row, column, scale and detection score.
	faces := classifier.RunCascade(cParams, 0)

	// Calculate the intersection over union (IoU) of two clusters.
	faces = classifier.ClusterDetections(faces, 0.18)

	//fmt.Println(faces)
	log.Printf("Detected faces in image: %v", len(faces))

	return faces, nil
}

// Get face coordinates
func GetFaceRect(faces []pigo.Detection) []image.Rectangle {
	var (
		qThresh float32 = 5.0
		rects   []image.Rectangle
	)

	for _, face := range faces {
		if face.Q > qThresh {
			rects = append(rects, image.Rect(
				face.Col-face.Scale/2, // Min.X
				face.Row-face.Scale/2, // Min.Y
				face.Col+face.Scale/2, // Max.X
				face.Row+face.Scale/2, // Max.Y
			))
		}
	}

	return rects
}

func getImage(filename string) (*image.NRGBA, string) {
	ext := filepath.Ext(filename)

	src, err := pigo.GetImage(filename)
	if err != nil {
		log.Fatalf("Cannot open the image file: %v", err)
	}
	return src, ext
}

// cropImage takes an image and crops it to the specified rectangle.
func cropImage(img image.Image, crop image.Rectangle) (image.Image, error) {
	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}

	// img is an Image interface. This checks if the underlying value has a
	// method called SubImage. If it does, then we can use SubImage to crop the
	// image.
	simg, ok := img.(subImager)
	if !ok {
		return nil, fmt.Errorf("image does not support cropping")
	}

	return simg.SubImage(crop), nil
}

func imageToBase64(file string) string {
	// Read the entire file into a byte slice
	bytes, err := os.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}

	var base64Encoding string
	// Determine the content type of the image file
	mimeType := http.DetectContentType(bytes)

	// Prepend the appropriate URI scheme header depending on the MIME type
	switch mimeType {
	case "image/jpeg":
		base64Encoding += "data:image/jpeg;base64,"
	case "image/png":
		base64Encoding += "data:image/png;base64,"
	default:
		log.Fatalf("mime type %v for %v is not supported.", mimeType, file)
	}

	// Append the base64 encoded output to image encoding
	base64Encoding += base64.StdEncoding.EncodeToString(bytes)

	// Return the full base64 representation of the image
	return base64Encoding
}

// Write image to a destination directory, respect file type encoding
func writeImage(img image.Image, dst_dir string, ext string) (string, error) {
	hash := fmt.Sprintf("%08x", crcHash(img, ext))
	log.Printf("Save cropped face to %v/%v%v", dst_dir, hash, ext)
	fd, err := os.Create(dst_dir + "/" + hash + ext)
	if err != nil {
		log.Fatal(err)
	}

	defer fd.Close()

	switch ext {
	case ".jpg", ".jpeg":
		jpeg.Encode(fd, img, &jpeg.Options{Quality: 95})
	case ".png":
		encoder := png.Encoder{CompressionLevel: 0}
		encoder.Encode(fd, img)
	default:
		return "", errors.New("unsupported image format")
	}
	base64_image := imageToBase64(dst_dir + "/" + hash + ext)

	return base64_image, nil
}

// Create CRC32 hash of image content for a unique file name
func crcHash(img image.Image, ext string) uint32 {
	crc32q := crc32.MakeTable(0xeb31d82e)
	buf := new(bytes.Buffer)
	switch ext {
	case ".jpg", ".jpeg":
		jpeg.Encode(buf, img, nil)
	case ".png":
		encoder := png.Encoder{CompressionLevel: 0}
		encoder.Encode(buf, img)
	}
	send_s3 := buf.Bytes()
	hash := crc32.Checksum(send_s3, crc32q)
	return hash
}

func main() {
	// Read images from "source" directory
	src_dir := "src"
	files, err := os.ReadDir(src_dir)
	if err != nil {
		log.Fatal(err)
	}

	// Get all images within a directory
	for _, file := range files {
		log.Printf("Processing image %v ...", file.Name())
		src, ext := getImage(src_dir + "/" + file.Name())
		switch ext {
		// Only some extensions are allowed
		case ".jpg", ".jpeg", ".png":
			faces, err := DetectFaces(src)
			if err != nil {
				log.Fatalf("Cannot detect faces: %v", err)
			}
			rects := GetFaceRect(faces)
			log.Println("rects resp:", rects)
			// Crop every found face from image
			for i, px := range rects {
				face_no := strconv.Itoa(i + 1)
				min_x := px.Min.X
				min_y := px.Min.Y
				max_x := px.Max.X
				max_y := px.Max.Y
				log.Println("Face no in image: ", face_no)
				log.Println("Cropped face coordinates: Min X: ", px.Min.X, "Min Y: ", px.Min.Y, "Max X: ", px.Max.X, "Max Y: ", px.Max.Y)

				img, err := cropImage(src, image.Rect(min_x, min_y, max_x, max_y))
				if err != nil {
					log.Printf("Error cropImage: %v", err)
				}
				dst_dir := "dst"
				base64image, err := writeImage(img, dst_dir, ext)
				if err != nil {
					log.Printf("Error writeImage: %v", err)
				}
				resp := DetectionResult{
					Faces:       rects,
					ImageBase64: base64image,
				}

				j, err := json.Marshal(resp)
				if err != nil {
					log.Fatalf("Marshal not possible: %v", err)
				}
				log.Println("Summarized found faces coordinates json response:", string(j))
			}
		default:
			fmt.Println(file.Name() + " skipped. Filetype not suppored.")
			continue
		}
	}

}
