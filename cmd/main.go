package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/mitchellh/mapstructure"
	"github.com/nagypeterjob-edu/application-values/pkg/hash"
	"github.com/nagypeterjob-edu/application-values/pkg/resource"
	"gopkg.in/yaml.v3"
)

var (
	// Poinst to the location of namespace and service information
	resourcesDir = ""
	// Destination directory
	destinationDir = ""
	// S3 Bucket which stores resources from previous deployments
	bucket = ""
	// S3 Bucket region
	region = ""
	// S3 key prefix pointing to values e.g: s3://example-blucket/<prefix>/value.yaml
	prefix = ""
	// Number of goroutines to spawn
	workers = -1
	// S3 client
	svc *s3.Client
)

type yml = map[string]interface{}

type global = map[string]yml

type service struct {
	serviceName string
	filename    string
	namespace   string
	content     io.Reader
}

func walkResources(root string, ctx context.Context) (<-chan string, <-chan error) {
	resources := make(chan string)
	errc := make(chan error, 1)

	go func() {
		defer close(resources)
		defer close(errc)
		errc <- filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("filepath %s not found", path)
			}

			if !info.Mode().IsRegular() {
				return nil
			}

			ext := filepath.Ext(path)
			if ext != ".yml" && ext != ".yaml" {
				return nil
			}

			select {
			case resources <- path:
			case <-ctx.Done():
				return errors.New("walk canceled")
			}

			return nil
		})
	}()
	return resources, errc
}

func merge(values yml, global yml) yml {
	// map[string]interface{} type
	yamlType := reflect.TypeOf(global)

	for key, rightVal := range global {
		if leftVal, present := values[key]; present {
			// if value is already present in resources/<namespace>/<values>.yaml
			// and also not a map type, do nothing
			if reflect.TypeOf(leftVal) != yamlType {
				continue
			}
			// if value is already present in resources/<namespace>/<values>.yaml,
			// and also a map type: continue next level
			values[key] = merge(leftVal.(yml), rightVal.(yml))
		} else {
			// if value is not yet present in resources/<namespace>/<values>.yaml, add it
			values[key] = rightVal
		}
	}
	return values
}

func parseYaml(path string) (yml, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf(ReadingResourceErrMsg, path, err))
	}

	ret := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(data), &ret)
	return ret, err
}

var (
	ReadingResourceErrMsg = "Error occured while reading resource %s, %s"
	ParsingResourceErrMsg = "Error occured while parsing resource %s, %s"
)

func createOutputDirectories() error {
	err := os.MkdirAll(destinationDir+"/resources", os.ModePerm)
	if err != nil {
		return err
	}
	err = os.MkdirAll(destinationDir+"/applications", os.ModePerm)
	if err != nil {
		return err
	}
	err = os.MkdirAll(destinationDir+"/pipelines", os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

func setupS3Client() *s3.Client {
	config, err := external.LoadDefaultAWSConfig()
	if err != nil {
		panic("Error loading default aws sdk config")
	}
	config.Region = region
	config.HTTPClient = &http.Client{
		Timeout: time.Second * 5,
	}

	return s3.New(config)
}

// Search for resource in S3 bucket & if it is present, compare remote resource's ETag with local hash
func (s *service) hasChanged(ctx context.Context) (*bool, error) {
	key := fmt.Sprintf("%s-%s.yaml", s.serviceName, s.namespace)
	if len(prefix) != 0 {
		key = filepath.Join(prefix, key)
	}

	request := svc.HeadObjectRequest(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	result, err := request.Send(ctx)
	if err != nil {
		// Somehow NotFound error is not an aerr (AWS Error), we cannot do type assertion
		if strings.HasPrefix(err.Error(), "NotFound") {
			// If resource is not found in the bucket, return true to generate spinnaker stuff
			return aws.Bool(true), nil
		}
		return nil, err
	}

	etag := *result.ETag
	// ETag string has quotes around the hash itself, remove them
	etag = etag[1 : len(etag)-1]

	// Calculate md5 hash for the current <service>.yaml
	hex, err := hash.CalculateHash(s.content)
	if err != nil {
		return nil, err
	}

	return aws.Bool(hex != etag), nil
}

func (s *service) generateSpinnakerStuff(merged yml) error {
	spinnaker := resource.SpinnakerConfig{}
	err := mapstructure.Decode(merged, &spinnaker)
	if err != nil {
		return err
	}

	// Applications

	app := resource.Application{
		ServiceName:       s.serviceName,
		KubernetesAccount: spinnaker.Chart.Parameters["kubernetesAccount"],
	}

	err = app.Write(destinationDir + "/applications")
	if err != nil {
		return err
	}

	// Pipelines

	pipeline := resource.Pipeline{
		ServiceName: s.serviceName,
		Namespace:   s.namespace,
		Spinnaker:   spinnaker,
	}

	err = pipeline.Write(destinationDir + "/pipelines")
	if err != nil {
		return err
	}

	fmt.Printf("Spinnaker application & pipeline template was generated for %s/%s.\n", s.namespace, s.serviceName)
	return nil
}

func main() {
	optionsSet := flag.NewFlagSet("optionsSet", flag.ExitOnError)
	optionsSet.StringVar(&resourcesDir, "values", "", "Values files location")
	optionsSet.StringVar(&destinationDir, "destination", "", "Generated files will end up here")
	optionsSet.StringVar(&bucket, "bucket", "", "S3 Bucket which stores resources from previous deployments")
	optionsSet.StringVar(&region, "region", "us-east-1", "S3 Bucket region")
	optionsSet.StringVar(&prefix, "prefix", "values", "S3 key prefix pointing to values e.g: s3://example-blucket/<prefix>/value.yaml")
	optionsSet.IntVar(&workers, "workers", -1, "Number of goroutines to spawn")
	err := optionsSet.Parse(os.Args[1:])
	if err != nil {
		panic("Error occured wile parsing flags")
	}

	if err := createOutputDirectories(); err != nil {
		panic(err)
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	svc = setupS3Client()

	// If not defined, use all cores
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	// Retrieve globals for each namespaces
	globalPaths, err := filepath.Glob(resourcesDir + "/**/global*")
	if err != nil {
		panic(fmt.Sprintf("Error locating globals %s", err))
	}

	globals := global{}
	for _, path := range globalPaths {
		directory := filepath.Dir(path)
		namespace := directory[len(resourcesDir)+1:]

		content, err := parseYaml(path)
		if err != nil {
			panic(fmt.Sprintf("Error parsing global %s", err))
		}

		globals[namespace] = content
	}

	// collect resource paths
	paths, errc := walkResources(resourcesDir, ctx)

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for path := range paths {
				directory := filepath.Dir(path)
				filename := filepath.Base(path)
				ext := filepath.Ext(path)
				namespace := directory[len(resourcesDir)+1:]
				serviceName := strings.TrimSuffix(filename, ext)

				if serviceName == "global" {
					continue
				}

				content, err := parseYaml(path)
				if err != nil {
					panic(fmt.Sprintf(ParsingResourceErrMsg, path, err))
				}

				// merge resources/<namespace>/<service>.yaml with namespace globals
				merged := merge(content, globals[namespace])

				values := resource.Values{}
				err = mapstructure.Decode(merged, &values)
				if err != nil {
					panic(err)
				}

				data, err := yaml.Marshal(values)
				if err != nil {
					panic(err)
				}

				s := service{
					serviceName: serviceName,
					filename:    filename,
					namespace:   namespace,
					content:     strings.NewReader(string(data)),
				}

				err = s.generateSpinnakerStuff(merged)
				if err != nil {
					cancelFunc()
					panic(err)
				}

				// If there is no bucket defined, regenerate all resources
				changed := true
				if len(bucket) != 0 {
					toggle, err := s.hasChanged(ctx)
					if err != nil {
						cancelFunc()
						panic(fmt.Sprintf("Error retrieving previous resources %s", err))
					}
					changed = *toggle
				}

				// if resource hasn't changed, no need to re-generate & upload
				// exit goroutine
				if !changed {
					return
				}

				fn := fmt.Sprintf("%s-%s.yaml", s.serviceName, s.namespace)
				err = ioutil.WriteFile(fmt.Sprintf("%s/resources/%s", destinationDir, fn), data, 0644)
				if err != nil {
					panic(err)
				}
				fmt.Printf("Helm values was generated for %s/%s.\n", s.namespace, s.serviceName)
			}
		}()
	}

	// Wait for each service generation to complete
	wg.Wait()

	if err = <-errc; err != nil {
		panic(err)
	}
}
