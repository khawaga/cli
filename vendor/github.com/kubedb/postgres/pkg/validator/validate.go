package validator

import (
	"errors"
	"fmt"
	"strings"

	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/pkg/docker"
	"github.com/kubedb/apimachinery/pkg/storage"
	amv "github.com/kubedb/apimachinery/pkg/validator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func ValidatePostgres(client kubernetes.Interface, postgres *api.Postgres) error {
	if postgres.Spec.Version == "" {
		return fmt.Errorf(`object 'Version' is missing in '%v'`, postgres.Spec)
	}

	version := fmt.Sprintf("%v-db", postgres.Spec.Version)
	if err := docker.CheckDockerImageVersion(docker.ImagePostgres, version); err != nil {
		return fmt.Errorf(`image %v:%v not found`, docker.ImagePostgres, version)
	}

	if postgres.Spec.Storage != nil {
		var err error
		if err = amv.ValidateStorage(client, postgres.Spec.Storage); err != nil {
			return err
		}
	}
	if postgres.Spec.Standby != "" {
		if strings.ToLower(string(postgres.Spec.Standby)) != "hot" &&
			strings.ToLower(string(postgres.Spec.Standby)) != "warm" {
			return fmt.Errorf(`configuration.Standby "%v" invalid`, postgres.Spec.Standby)
		}
	}
	if postgres.Spec.Streaming != "" {
		// TODO: synchronous Streaming is unavailable due to lack of support
		if /*strings.ToLower(configuration.Streaming) != "synchronous" &&
		 */strings.ToLower(string(postgres.Spec.Streaming)) != "asynchronous" {
			return fmt.Errorf(`configuration.Streaming "%v" invalid`, postgres.Spec.Streaming)
		}
	}

	archiver := postgres.Spec.Archiver.Storage
	if archiver != nil {
		if archiver.StorageSecretName == "" {
			return fmt.Errorf(`object 'StorageSecretName' is missing in '%v'`, archiver)
		}
		if archiver.S3 == nil {
			return errors.New("no storage provider is configured")
		}
		if !(archiver.GCS == nil && archiver.Azure == nil && archiver.Swift == nil && archiver.Local == nil) {
			return errors.New("invalid storage provider is configured")
		}

		if err := storage.CheckBucketAccess(client, *archiver, postgres.Namespace); err != nil {
			return err
		}
	}

	databaseSecret := postgres.Spec.DatabaseSecret
	if databaseSecret != nil {
		if _, err := client.CoreV1().Secrets(postgres.Namespace).Get(databaseSecret.SecretName, metav1.GetOptions{}); err != nil {
			return err
		}
	}

	if postgres.Spec.Init != nil && postgres.Spec.Init.PostgresWAL != nil {
		wal := postgres.Spec.Init.PostgresWAL
		if wal.StorageSecretName == "" {
			return fmt.Errorf(`object 'StorageSecretName' is missing in '%v'`, wal)
		}
		if wal.S3 == nil {
			return errors.New("no storage provider is configured")
		}
		if !(wal.GCS == nil && wal.Azure == nil && wal.Swift == nil && wal.Local == nil) {
			return errors.New("invalid storage provider is configured")
		}

		if err := storage.CheckBucketAccess(client, wal.SnapshotStorageSpec, postgres.Namespace); err != nil {
			return err
		}
	}

	backupScheduleSpec := postgres.Spec.BackupSchedule
	if backupScheduleSpec != nil {
		if err := amv.ValidateBackupSchedule(client, backupScheduleSpec, postgres.Namespace); err != nil {
			return err
		}
	}

	monitorSpec := postgres.Spec.Monitor
	if monitorSpec != nil {
		if err := amv.ValidateMonitorSpec(monitorSpec); err != nil {
			return err
		}

	}
	return nil
}