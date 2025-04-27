package main

import (
	"github.com/kobu/repo-int/pkg/app"
)

func main() {
	runtimeApp := app.NewApp()
	runtimeApp.Run()

	// cfg, err := config.LoadDefaultConfig(context.TODO(),
	// 	config.WithRegion("eu-north-1"),
	// 	config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("", "", "")))
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// aws.ListBucketContent(cfg, "c8y-kobu-device-artifacts-repository")
	// aws.GetPresignURL(cfg, "c8y-kobu-device-artifacts-repository", "my-first-software.txt")
}
