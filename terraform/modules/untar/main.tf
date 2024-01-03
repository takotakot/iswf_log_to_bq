data "google_project" "project" {}

resource "google_project_service" "functions" {
  service            = "cloudfunctions.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "cloudbuild" {
  service            = "cloudbuild.googleapis.com"
  disable_on_destroy = false
}

resource "google_service_account" "default" {
  account_id   = "untar-function-sa"
  display_name = "Untar Fuction Service Account"
}

resource "google_storage_bucket" "output_bucket" {
  name                        = var.output_bucket
  location                    = "US"
  uniform_bucket_level_access = true
}

data "google_storage_project_service_account" "gcs_account" {
}

# Grant permission to invoke Cloud Run services
resource "google_project_iam_member" "runinvoker" {
  project    = data.google_project.project.id
  role       = "roles/run.invoker"
  member     = "serviceAccount:${google_service_account.default.email}"
}

resource "google_project_iam_member" "event-receiving" {
  depends_on = [google_project_iam_member.runinvoker]
  project = data.google_project.project.id
  role    = "roles/eventarc.eventReceiver"
  member  = "serviceAccount:${google_service_account.default.email}"
}

resource "google_project_iam_member" "artifactregistry-reader" {
  depends_on = [google_project_iam_member.event-receiving]
  project    = data.google_project.project.id
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:${google_service_account.default.email}"
}

resource "google_storage_bucket_iam_member" "object-input" {
  bucket     = var.tar_bucket
  role       = "roles/storage.objectUser"
  member     = "serviceAccount:${google_service_account.default.email}"
}

resource "google_storage_bucket_iam_member" "object-output" {
  bucket     = google_storage_bucket.output_bucket.name
  role       = "roles/storage.objectUser"
  member     = "serviceAccount:${google_service_account.default.email}"
}

resource "google_pubsub_topic" "notify_topic" {
  name = var.notify_topic
}

resource "google_pubsub_topic_iam_member" "publish" {
  project = data.google_project.project.name
  topic   = google_pubsub_topic.notify_topic.name
  role    = "roles/pubsub.publisher"
  member  = "serviceAccount:${google_service_account.default.email}"
}

resource "google_cloudfunctions2_function" "default" {
  depends_on = [
    google_project_service.functions,
    google_project_service.cloudbuild,
    google_project_iam_member.event-receiving,
    google_project_iam_member.artifactregistry-reader,
  ]
  lifecycle {
    ignore_changes = [
      service_config[0].service,
      service_config[0].service_account_email,
      build_config[0].entry_point,
    ]
  }

  name        = "untar"
  description = "Untar file content from tar_bucket"
  location    = "us-central1"

  build_config {
    entry_point = "template"
    runtime     = "go121"
    source {
      storage_source {
        bucket = var.source_archive_bucket
        object = var.source_archive_object
      }
    }
  }
  service_config {
    available_memory = "256M"
    environment_variables = {
      CONTENT_TOPIC_ID = google_pubsub_topic.notify_topic.name
      DEST_BUCKET_NAME = google_storage_bucket.output_bucket.name
      PROJECT_ID       = data.google_project.project.project_id
    }
    ingress_settings                 = "ALLOW_ALL"
    max_instance_count               = 1
    max_instance_request_concurrency = 1
    min_instance_count               = 0
    timeout_seconds                  = 60
    service_account_email            = google_service_account.default.email
  }
  event_trigger {
    trigger_region        = "us-central1"
    event_type            = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic          = var.source_topic_id
    retry_policy          = "RETRY_POLICY_RETRY"
    service_account_email = google_service_account.default.email
  }
}
