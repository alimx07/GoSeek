package config

import (
	"os"
)

type GlobalConfig struct {
	ChunkSize             int
	NumWorkers            int
	IndexBatchMemoryLimit int32
	ChannelBufferSize     int
}

// Global configs of the app
func LoadGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		ChunkSize:             1 * 1024 * 1024,
		NumWorkers:            4,
		IndexBatchMemoryLimit: 32 * 1024 * 1024,
		ChannelBufferSize:     16,
	}
}

// use txt file as the presistent memory for now
func SaveToFile(filePath string) error {
	data := []byte(filePath + "\n")
	if _, err := os.Stat("indexes.txt"); os.IsNotExist(err) {
		return os.WriteFile("indexes.txt", data, 0644)
	}

	file, err := os.OpenFile("indexes.txt", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	return err
}

func ReadFromFile() (string, error) {
	data, err := os.ReadFile("indexes.txt")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
