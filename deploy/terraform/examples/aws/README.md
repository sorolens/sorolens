# Example: Sorolens on AWS

A complete root module around [modules/aws](../../modules/aws): provider,
region, tags and (commented) remote state backend.

```sh
cp terraform.tfvars.example terraform.tfvars   # then edit it
terraform init
terraform plan
terraform apply
```

Follow the [AWS install guide](../../../../docs/self-hosting/aws.md) for the
full walkthrough (images, HTTPS, migrations, teardown and costs). It has not
yet been run against a real AWS account.

The `module` source here is a relative path so the example always tracks this
repository. When you copy it elsewhere, pin a revision instead:

```hcl
source = "github.com/sorolens/sorolens//deploy/terraform/modules/aws?ref=<commit or tag>"
```

`tests/` plans this example against mocked providers (`terraform test`, no
credentials needed).
