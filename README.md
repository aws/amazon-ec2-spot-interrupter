# Amazon EC2 Spot Interrupter
The `ec2-spot-interrupter` is a simple CLI tool that triggers Amazon EC2 Spot Interruption Notifications and Rebalance Recommendations.

[![Actions Status](https://github.com/aws/amazon-ec2-spot-interrupter/workflows/Go/badge.svg)](https://github.com/aws/amazon-ec2-spot-interrupter/actions)


## Installation

```bash
brew tap aws/tap
brew install aws/ec2-spot-interrupter
```

## About

[Amazon EC2 Spot](https://aws.amazon.com/ec2/spot/) Instances let you run flexible, fault-tolerant, or stateless applications in the AWS Cloud at up to a 90% discount from On-Demand prices. 
Spot instances are regular EC2 capacity that can be reclaimed by AWS with a 2-minute notification called the [Interruption Notification](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-interruptions.html).
Applications that are able to gracefully handle this notification and respond by check pointing or draining work can leverage Spot for deeply discounted compute resources! In addition to Interruption Notifications, [Rebalance Recommendation Events](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/rebalance-recommendations.html) are sent to spot instances that are at higher risk of being interrupted. Handling Rebalance Recommendations can potentially give your application even more time to gracefully shutdown than the 2 minutes an Interruption Notification would give you.

It can be challenging to test your application's handling of Spot Interruption Notifications and Rebalance Recommendations. The [AWS Fault Injection Simulator](https://aws.amazon.com/fis/) (FIS) supports sending real Spot Interruptions and Rebalance Recommendations to your spot instances so that you can test how your application responds. However, since FIS is a general purpose fault injection simulation service, it can be cumbersome to setup the required fault injection experiment templates to execute experiments for Spot. The `ec2-spot-interrupter` CLI tool streamlines this process as it wraps FIS and allows you to simply pass a list of instance IDs which `ec2-spot-interrupter` will use to craft the required experiment templates and then execute those experiments.

For details on how to use the AWS Fault Injection Simulator directly to trigger Spot Interruption Notifications, checkout this [blog post](https://aws.amazon.com/blogs/compute/implementing-interruption-tolerance-in-amazon-ec2-spot-with-aws-fault-injection-simulator/).

If you are looking for a tool to test Spot Interruption Notifications and Rebalance Recommendations locally on your laptop (not EC2), then checkout the [EC2 Metadata Mock](https://github.com/aws/amazon-ec2-metadata-mock).

## Usage

```bash
$ ec2-spot-interrupter is a simple CLI tool that triggers Amazon EC2 Spot Instance Interruption Notifications and Rebalance Recommendations.

Usage:
  ec2-spot-interrupter [flags]

Flags:
  -c, --clean                  clean up the underlying simulations (default true)
  -d, --delay duration         duration until the interruption notification is sent (default 15s)
  -h, --help                   help for ec2-spot-interrupter
  -i, --instance-ids strings   instance IDs to interrupt
      --interactive            interactive TUI
  -p, --profile string         the AWS Profile
  -r, --region string          the AWS Region
  -v, --version                the version
```

Try the interactive TUI mode:

```bash
$ ec2-spot-interrupter --interactive
```

Or use the regular CLI options:

```bash
$ ec2-spot-interrupter --instance-ids i-0208a716009d70b36
===================================================================
📖 Experiment Summary:
        ID: EXPBCcSv1NvRNTek58
  Role ARN: arn:aws:iam::1234567890:role/aws-fis-itn
    Action: aws:ec2:send-spot-instance-interruptions
   Targets:
    - i-0208a716009d70b36
===================================================================
2022-05-18T11:39:45: ✅ Rebalance Recommendation sent
2022-05-18T11:39:45: ⏳ Interruption will be sent in 15 seconds
2022-05-18T11:40:05: ✅ Spot 2-minute Interruption Notification sent
2022-05-18T11:42:05: ✅ Spot Instance Shutdown sent
```

## Communication
If you've run into a bug or have a new feature request, please open an [issue](https://github.com/aws/amazon-ec2-spot-interrupter/issues/new).

Check out the open source [Amazon EC2 Spot Instances Integrations Roadmap](https://github.com/aws/ec2-spot-instances-integrations-roadmap) to see what we're working on and give us feedback! 

##  Contributing
Contributions are welcome! Please read our [guidelines](https://github.com/aws/amazon-ec2-spot-interrupter/blob/main/CONTRIBUTING.md) and our [Code of Conduct](https://github.com/aws/itn/blob/main/CODE_OF_CONDUCT.md).

## License
This project is licensed under the [Apache-2.0](LICENSE) License.
