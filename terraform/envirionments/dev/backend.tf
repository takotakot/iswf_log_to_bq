terraform {
  backend "gcs" {
    bucket = "hg-iswf-log-dev_tfstate"
  }
}
