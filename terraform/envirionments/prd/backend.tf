terraform {
  backend "gcs" {
    bucket = "hg-iswf-log-prd_tfstate"
  }
}
