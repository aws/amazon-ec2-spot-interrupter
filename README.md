# itn
`itn` is a simple CLI tool that triggers Amazon EC2 Spot Interruption Termination Notifications (ITNs) and Rebalance Recommendations.

[![Actions Status](https://github.com/aws/itn/workflows/Go/badge.svg)](https://github.com/aws/itn/actions)


## Installation

```
brew tap aws/tap
brew install aws/itn
```

## About

[Amazon EC2 Spot](https://aws.amazon.com/ec2/spot/) Instances let you run flexible, fault-tolerant, or stateless applications in the AWS Cloud at up to a 90% discount from On-Demand prices. 
Spot instances are regular EC2 capacity that can be reclaimed by AWS with a 2-minute notification called the [Interruption Termination Notification (ITN)](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-interruptions.html).
Applications that are able to gracefully handle this notification and respond by checking pointing or draining work can leverage Spot for deeply discounted compute resources! In addition to ITNs, [Rebalance Recommendation Events](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/rebalance-recommendations.html) are sent to spot instances that are at higher risk of receiving an ITN. Handling Rebalance Recommendations can potentially give your application even more time to gracefully shutdown than the 2 minutes an ITN would give you.

It can be challening to test your application's handling of Spot ITNs and Rebalance Recommendations. The [AWS Fault Injection Simulator](https://aws.amazon.com/fis/) (FIS) supports sending real spot ITNs and rebalance recommendations to your spot instances so that you can test how your application responds. Since FIS is a general purpose fault injection simulation service, it can be cumbersome to setup the required fault injection experiment templates and execute experiments for Spot. To make it easy, the `itn` CLI tool wraps FIS and allows you to simply pass a list of instance IDs which `itn` will use to craft the required experiment templates and then execute those experiments.

For details on how to use the AWS Fault Injection Simulator directly to trigger Spot ITNs, checkout this [blog post](https://aws.amazon.com/blogs/compute/implementing-interruption-tolerance-in-amazon-ec2-spot-with-aws-fault-injection-simulator/).

If you are looking for a tool to test Spot ITNs and Rebalance Recommendations locally on your laptop (not EC2), then checkout the [EC2 Metadata Mock](https://github.com/aws/amazon-ec2-metadata-mock).

## Usage

```
$ itn --help
itn is a simple CLI tool that triggers Amazon EC2 Spot Interruption Termination Notifications (ITNs) and Rebalance Recommendations.

Usage:
  itn [flags]

Flags:
  -h, --help                   help for itn
  -i, --instance-ids strings   instance IDs to interrupt
  -v, --version                the version
```

```
$ itn --instance-ids i-0123456789 i-9876543210
✅ Successfully sent spot rebalance recommendation and ITN to [i-0123456789 i-9876543210]
```

## Communication
If you've run into a bug or have a new feature request, please open an [issue](https://github.com/aws/itn/issues/new).

Check out the open source [Amazon EC2 Spot Instances Integrations Roadmap](https://github.com/aws/ec2-spot-instances-integrations-roadmap) to see what we're working on and give us feedback! 

##  Contributing
Contributions are welcome! Please read our [guidelines](https://github.com/aws/itn/blob/main/CONTRIBUTING.md) and our [Code of Conduct](https://github.com/aws/itn/blob/main/CODE_OF_CONDUCT.md).

## License
This project is licensed under the [Apache-2.0](LICENSE) License.
