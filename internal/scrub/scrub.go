package scrub

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"net/http"
	"strings"
)

// MetadataReport holds extracted EXIF, GPS, and device information
type MetadataReport struct {
	Filename       string            `json:"filename"`
	FileSize       int64             `json:"file_size"`
	FileSizeHuman  string            `json:"file_size_human"`
	MimeType       string            `json:"mime_type"`
	Width          int               `json:"width"`
	Height         int               `json:"height"`
	HasGPS         bool              `json:"has_gps"`
	Latitude       float64           `json:"latitude,omitempty"`
	Longitude      float64           `json:"longitude,omitempty"`
	AltitudeMeters float64           `json:"altitude_meters,omitempty"`
	MapsURL        string            `json:"maps_url,omitempty"`
	CameraMake     string            `json:"camera_make,omitempty"`
	CameraModel    string            `json:"camera_model,omitempty"`
	Software       string            `json:"software,omitempty"`
	DateTaken      string            `json:"date_taken,omitempty"`
	TotalTagsFound int               `json:"total_tags_found"`
	RawTags        map[string]string `json:"raw_tags"`
}

// CleanReport provides before/after metrics following metadata removal
type CleanReport struct {
	Filename       string  `json:"filename"`
	OriginalSize   int64   `json:"original_size"`
	CleanedSize    int64   `json:"cleaned_size"`
	SavedBytes     int64   `json:"saved_bytes"`
	SavedPercent   float64 `json:"saved_percent"`
	TagsStripped   int     `json:"tags_stripped"`
	MimeType       string  `json:"mime_type"`
	CleanedDataB64 string  `json:"cleaned_data_b64,omitempty"`
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// InspectMetadata extracts EXIF, GPS, and device parameters from raw image bytes
func InspectMetadata(data []byte, filename string) (*MetadataReport, error) {
	mimeType := http.DetectContentType(data)
	if filename == "" {
		filename = "image"
	}

	report := &MetadataReport{
		Filename:      filename,
		FileSize:      int64(len(data)),
		FileSizeHuman: formatBytes(int64(len(data))),
		MimeType:      mimeType,
		RawTags:       make(map[string]string),
	}

	// Try image config decode for dimensions
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err == nil {
		report.Width = cfg.Width
		report.Height = cfg.Height
	}

	// Parse JPEG EXIF
	if strings.HasPrefix(mimeType, "image/jpeg") || bytes.HasPrefix(data, []byte{0xFF, 0xD8}) {
		parseJPEGExif(data, report)
	} else if strings.HasPrefix(mimeType, "image/png") || bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47}) {
		parsePNGMetadata(data, report)
	}

	report.TotalTagsFound = len(report.RawTags)
	return report, nil
}

// CleanImage re-encodes the image into a clean byte stream completely devoid of EXIF and metadata chunks
func CleanImage(data []byte, filename string) ([]byte, *CleanReport, error) {
	inspect, _ := InspectMetadata(data, filename)
	tagsCount := 0
	if inspect != nil {
		tagsCount = inspect.TotalTagsFound
	}

	// Decode source image
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode image: %w", err)
	}

	var out bytes.Buffer
	outMime := "image/jpeg"

	switch strings.ToLower(format) {
	case "png":
		outMime = "image/png"
		enc := png.Encoder{CompressionLevel: png.BestCompression}
		if err := enc.Encode(&out, img); err != nil {
			return nil, nil, fmt.Errorf("failed to re-encode clean PNG: %w", err)
		}
	case "gif":
		outMime = "image/gif"
		// Encode as PNG or high-quality JPEG if single frame
		if err := png.Encode(&out, img); err != nil {
			return nil, nil, fmt.Errorf("failed to re-encode clean image: %w", err)
		}
	default:
		// Default to JPEG with high quality (92)
		outMime = "image/jpeg"
		opts := &jpeg.Options{Quality: 92}
		if err := jpeg.Encode(&out, img, opts); err != nil {
			return nil, nil, fmt.Errorf("failed to re-encode clean JPEG: %w", err)
		}
	}

	cleanedBytes := out.Bytes()
	origSize := int64(len(data))
	cleanSize := int64(len(cleanedBytes))
	savedBytes := origSize - cleanSize
	savedPct := 0.0
	if origSize > 0 && savedBytes > 0 {
		savedPct = (float64(savedBytes) / float64(origSize)) * 100.0
	}

	report := &CleanReport{
		Filename:     filename,
		OriginalSize: origSize,
		CleanedSize:  cleanSize,
		SavedBytes:   savedBytes,
		SavedPercent: math.Round(savedPct*10) / 10,
		TagsStripped: tagsCount,
		MimeType:     outMime,
	}

	return cleanedBytes, report, nil
}

// parseJPEGExif searches for APP1 marker (0xFF, 0xE1) and parses standard TIFF tags
func parseJPEGExif(data []byte, report *MetadataReport) {
	idx := 2 // Skip SOI 0xFF, 0xD8
	for idx < len(data)-4 {
		if data[idx] != 0xFF {
			idx++
			continue
		}
		marker := data[idx+1]
		if marker == 0xDA || marker == 0xD9 { // SOS or EOI
			break
		}
		length := int(binary.BigEndian.Uint16(data[idx+2 : idx+4]))
		if idx+2+length > len(data) {
			break
		}

		// APP1 Marker: 0xE1
		if marker == 0xE1 {
			payload := data[idx+4 : idx+2+length]
			if len(payload) > 6 && string(payload[:4]) == "Exif" {
				parseTIFFBlock(payload[6:], report)
			}
		}
		idx += 2 + length
	}
}

// parseTIFFBlock parses TIFF header and IFD entries
func parseTIFFBlock(tiff []byte, report *MetadataReport) {
	if len(tiff) < 8 {
		return
	}

	var byteOrder binary.ByteOrder
	if tiff[0] == 'I' && tiff[1] == 'I' {
		byteOrder = binary.LittleEndian
	} else if tiff[0] == 'M' && tiff[1] == 'M' {
		byteOrder = binary.BigEndian
	} else {
		return
	}

	firstIFDOffset := byteOrder.Uint32(tiff[4:8])
	if int(firstIFDOffset) >= len(tiff) {
		return
	}

	var gpsOffset uint32
	parseIFD(tiff, int(firstIFDOffset), byteOrder, report, &gpsOffset)

	// If GPS sub-IFD exists, parse it
	if gpsOffset > 0 && int(gpsOffset) < len(tiff) {
		parseGPSIFD(tiff, int(gpsOffset), byteOrder, report)
	}
}

func parseIFD(tiff []byte, offset int, order binary.ByteOrder, report *MetadataReport, gpsOffset *uint32) {
	if offset+2 > len(tiff) {
		return
	}
	numEntries := int(order.Uint16(tiff[offset : offset+2]))
	offset += 2

	for i := 0; i < numEntries; i++ {
		entryOffset := offset + (i * 12)
		if entryOffset+12 > len(tiff) {
			break
		}
		tag := order.Uint16(tiff[entryOffset : entryOffset+2])
		typ := order.Uint16(tiff[entryOffset+2 : entryOffset+4])
		count := order.Uint32(tiff[entryOffset+4 : entryOffset+8])
		valBytes := tiff[entryOffset+8 : entryOffset+12]

		strVal := readTagValueString(tiff, typ, count, valBytes, order)

		switch tag {
		case 0x010F: // Make
			report.CameraMake = strVal
			report.RawTags["Camera Make"] = strVal
		case 0x0110: // Model
			report.CameraModel = strVal
			report.RawTags["Camera Model"] = strVal
		case 0x0131: // Software
			report.Software = strVal
			report.RawTags["Software / OS"] = strVal
		case 0x0132: // DateTime
			report.DateTaken = strVal
			report.RawTags["Date / Time Modified"] = strVal
		case 0x9003: // DateTimeOriginal
			report.DateTaken = strVal
			report.RawTags["Date Taken (Original)"] = strVal
		case 0x8825: // GPSInfo IFD Pointer
			*gpsOffset = order.Uint32(valBytes)
		case 0x829A:
			report.RawTags["Exposure Time"] = strVal
		case 0x829D:
			report.RawTags["F-Number / Aperture"] = strVal
		case 0x8827:
			report.RawTags["ISO Speed"] = strVal
		case 0x920A:
			report.RawTags["Focal Length"] = strVal
		}
	}
}

func parseGPSIFD(tiff []byte, offset int, order binary.ByteOrder, report *MetadataReport) {
	if offset+2 > len(tiff) {
		return
	}
	numEntries := int(order.Uint16(tiff[offset : offset+2]))
	offset += 2

	var latRef, lonRef string
	var latCoords, lonCoords []float64
	var altitude float64

	for i := 0; i < numEntries; i++ {
		entryOffset := offset + (i * 12)
		if entryOffset+12 > len(tiff) {
			break
		}
		tag := order.Uint16(tiff[entryOffset : entryOffset+2])
		_ = order.Uint16(tiff[entryOffset+2 : entryOffset+4])
		count := order.Uint32(tiff[entryOffset+4 : entryOffset+8])
		valBytes := tiff[entryOffset+8 : entryOffset+12]

		switch tag {
		case 0x0001: // GPSLatitudeRef
			latRef = strings.TrimSpace(string(valBytes[:1]))
			report.RawTags["GPS Latitude Ref"] = latRef
		case 0x0002: // GPSLatitude (3 RATIONALs)
			latCoords = readRationals(tiff, count, valBytes, order)
		case 0x0003: // GPSLongitudeRef
			lonRef = strings.TrimSpace(string(valBytes[:1]))
			report.RawTags["GPS Longitude Ref"] = lonRef
		case 0x0004: // GPSLongitude (3 RATIONALs)
			lonCoords = readRationals(tiff, count, valBytes, order)
		case 0x0006: // GPSAltitude (1 RATIONAL)
			alt := readRationals(tiff, 1, valBytes, order)
			if len(alt) > 0 {
				altitude = alt[0]
				report.AltitudeMeters = altitude
				report.RawTags["GPS Altitude"] = fmt.Sprintf("%.2f meters", altitude)
			}
		}
	}

	if len(latCoords) == 3 && len(lonCoords) == 3 {
		lat := latCoords[0] + (latCoords[1] / 60.0) + (latCoords[2] / 3600.0)
		if strings.EqualFold(latRef, "S") {
			lat = -lat
		}

		lon := lonCoords[0] + (lonCoords[1] / 60.0) + (lonCoords[2] / 3600.0)
		if strings.EqualFold(lonRef, "W") {
			lon = -lon
		}

		report.HasGPS = true
		report.Latitude = math.Round(lat*1000000) / 1000000
		report.Longitude = math.Round(lon*1000000) / 1000000
		report.MapsURL = fmt.Sprintf("https://www.openstreetmap.org/?mlat=%.6f&mlon=%.6f#map=16/%.6f/%.6f", report.Latitude, report.Longitude, report.Latitude, report.Longitude)

		report.RawTags["GPS Coordinates"] = fmt.Sprintf("%.6f, %.6f", report.Latitude, report.Longitude)
	}
}

func readTagValueString(tiff []byte, typ uint16, count uint32, valBytes []byte, order binary.ByteOrder) string {
	if typ == 2 { // ASCII
		if count <= 4 {
			return strings.TrimRight(string(valBytes[:count]), "\x00")
		}
		offset := int(order.Uint32(valBytes))
		if offset+int(count) <= len(tiff) {
			return strings.TrimRight(string(tiff[offset:offset+int(count)]), "\x00")
		}
	}
	return ""
}

func readRationals(tiff []byte, count uint32, valBytes []byte, order binary.ByteOrder) []float64 {
	offset := int(order.Uint32(valBytes))
	var res []float64
	for i := 0; i < int(count); i++ {
		entry := offset + (i * 8)
		if entry+8 > len(tiff) {
			break
		}
		num := float64(order.Uint32(tiff[entry : entry+4]))
		den := float64(order.Uint32(tiff[entry+4 : entry+8]))
		if den != 0 {
			res = append(res, num/den)
		} else {
			res = append(res, 0)
		}
	}
	return res
}

// parsePNGMetadata extracts text comments and chunks from PNG format
func parsePNGMetadata(data []byte, report *MetadataReport) {
	idx := 8 // Skip PNG signature
	for idx < len(data)-8 {
		length := int(binary.BigEndian.Uint32(data[idx : idx+4]))
		chunkType := string(data[idx+4 : idx+8])
		payloadIdx := idx + 8
		if payloadIdx+length > len(data) {
			break
		}

		if chunkType == "tEXt" || chunkType == "iTXt" {
			payload := data[payloadIdx : payloadIdx+length]
			nullIdx := bytes.IndexByte(payload, 0)
			if nullIdx > 0 && nullIdx < len(payload)-1 {
				key := string(payload[:nullIdx])
				val := string(payload[nullIdx+1:])
				val = strings.TrimSpace(val)
				report.RawTags[key] = val
				if strings.EqualFold(key, "Software") {
					report.Software = val
				}
				if strings.EqualFold(key, "Creation Time") {
					report.DateTaken = val
				}
			}
		}

		if chunkType == "IEND" {
			break
		}
		idx += 12 + length
	}
}

// Helper to check if file has any metadata
func HasMetadata(data []byte) bool {
	r, err := InspectMetadata(data, "")
	if err != nil {
		return false
	}
	return r.TotalTagsFound > 0
}
