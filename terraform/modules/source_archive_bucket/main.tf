resource "google_storage_bucket" "source-archive-bucket" {
  name     = var.source_archive_bucket
  location = "US"

  versioning {
    enabled = true
  }

  lifecycle_rule {
    condition {
      days_since_noncurrent_time = 7
    }
    action {
      type = "Delete"
    }
  }
}
