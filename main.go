package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/nguyenthenguyen/docx"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type copiaProforma struct {
	indirizzoTicket string `json:"indirizzoTicket"`
	indirizzo       string `json:"indirizzo"`
	citta           string `json:"citta"`
	descrLavoro     string `json:"descrLavoro"`
	ragioneSociale  string `json:"ragioneSociale"`
	piva            string `json:"piva"`
	numTel          string `json:"numTel"`
	ricambiForniti  string `json:"ricambiForniti"`
	pz              string `json:"pz"`
	prezzo          string `json:"prezzo"`
	inizioLavoro    string `json:"inizioLavoro"`
	fineLavoro      string `json:"fineLavoro"`
	tot             string `json:"tot"`
	idTicket        int    `json:"idTicket"`
}

type proformaReturn struct {
	receiptURL string `json:"receiptURL"`
	message    string `json:"message"`
}

func init() {

	godotenv.Load(".env")

}

func HandleRequest(ctx context.Context, temp copiaProforma) (string, error) {

	bucketname := os.Getenv("BUCKET_NAME")
	key := os.Getenv("OBJECT_KEY")

	jsonTemp, _ := json.MarshalIndent(temp, "", "  ")
	log.Printf("INPUT: %s", jsonTemp)

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Profile: "default",
		Config: aws.Config{
			Region:                        aws.String(endpoints.EuCentral1RegionID),
			CredentialsChainVerboseErrors: aws.Bool(true),
		},
		SharedConfigState: session.SharedConfigEnable,
	}))
	creds := stscreds.NewCredentials(sess, *aws.String(os.Getenv("ARN_ROLE")))
	svc := s3.New(sess, &aws.Config{Credentials: creds, CredentialsChainVerboseErrors: aws.Bool(true)})

	cotx := context.Background()

	result, err := svc.GetObjectWithContext(cotx, &s3.GetObjectInput{
		Bucket: aws.String(bucketname),
		Key:    aws.String(key),
	})
	if err != nil {
		// Cast err to awserr.Error to handle specific error codes.
		aerr, ok := err.(awserr.Error)
		if ok && aerr.Code() == s3.ErrCodeNoSuchKey {
			// Specific error code handling
		}
		fmt.Println("error: ", err)
	}

	dst, err := os.Create(filepath.Join("./temp", filepath.Base(key)))
	if err != nil {
		log.Fatal("error: ", err)
	}

	if _, err = io.Copy(dst, result.Body); err != nil {
		log.Fatal("error: ", err)
	}

	dst.Close()

	r, err := docx.ReadDocxFile(filepath.Join("./temp", filepath.Base(key)))
	if err != nil {
		log.Fatal(err)
	}

	doc := r.Editable()

	doc.Replace("ragioneSociale", temp.ragioneSociale, 1)
	doc.Replace("indirizzoTicket", temp.indirizzoTicket, 1)
	doc.Replace("indirizzo", temp.indirizzo, 1)
	doc.Replace("citta", temp.citta, 1)
	doc.Replace("numTel", temp.numTel, 1)
	doc.Replace("piva", temp.piva, 1)
	doc.Replace("inizioLavoro", temp.inizioLavoro, 1)
	doc.Replace("fineLavoro", temp.fineLavoro, 1)
	doc.Replace("tot", temp.tot, 1)
	doc.Replace("pz", temp.pz, 1)
	doc.Replace("prezzo", temp.prezzo, 1)

	newKey := *aws.String(strconv.Itoa(temp.idTicket)) + *aws.String(time.Now().Format("01-02-2006 15.04.05")) + key

	if err := doc.WriteToFile(filepath.Join("./final", filepath.Base(newKey))); err != nil {
		log.Fatal(err)
	}

	f, err := os.Open(filepath.Join("./final", filepath.Base(newKey)))
	if err != nil {
		log.Fatal(err)
	}

	up := s3manager.NewUploader(sess)

	if _, err := up.Upload(&s3manager.UploadInput{
		Bucket: &bucketname,
		Key:    &newKey,
		Body:   f,
	}); err != nil {
		log.Fatal(err)
	}

	f.Close()
	r.Close()

	e := os.Remove(filepath.Join("./temp", filepath.Base(key)))
	if e != nil {
		log.Fatal(e)
	}

	removeErr := os.Remove(filepath.Join("./final", filepath.Base(newKey)))
	if removeErr != nil {
		log.Fatal(removeErr)
	}

	// Make sure to close the body when done with it for S3 GetObject APIs or
	// will leak connections.
	defer result.Body.Close()

	fmt.Println("Object Size:", aws.Int64Value(result.ContentLength))
	//return err

	return fmt.Sprintf("Hello %s!", temp.ragioneSociale), nil
}

func main() {

	lambda.Start(HandleRequest)
}
