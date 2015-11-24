---
title: Exposing Mesos-DNS Services Externally
---


# Exposing Mesos-DNS Services Externally

Mesos-DNS is primarily useful within a Mesos or DCOS cluster for service discovery, but unfortunately, sometimes we have services which do not run on Mesos, but yet they would like to consume services on Mesos. This tutorial describes one mechanism by which to enable downstreams outside of Mesos to consume services on Mesos.


## Problem Statement
The way that Mesos-DNS is typically deployed, especially on DCOS, is Mesos-DNS replaces the normal resolvers in /etc/resolv.conf. Then, it points to the user's original recursive DNS server. This enables all agents to utilize Mesos-DNS as their service discovery mechanism, but unfortunately, it renders machines that are not using Mesos-DNS as their default resolver unable to properly work.


<div>
  <p>
    <img src="{{ site.baseurl}}/img/graffles/BasicQuery4.png" style="height: auto; max-width: 100%;" width="610" height="320" alt="">
    <em>External Services not working in a typical Mesos-DNS cluster</em>
  </p>
</div>


## Workaround
We can work around this fortunately. It requires some new infrastructure to be deployed, and configs to be changed, but it is absolutely possible.

### Steps:
1. Choose a domain to expose your internal services on.

    By default, Mesos-DNS exposes services on `.mesos`. This is great within a DCOS cluster, but unfortunately, it's a TLD, and not in your organization's domain. Let's say that we work at Widget Co, Inc, and we own widgets.com. In addition to this, let's that we have one datacenter. **SFO1**. It runs a cluster of DNS servers with the domain name `sfo.widgets.com`.

   We want to make the services in that cluster available to machines that are not using Mesos-DNS as their resolver. So, let's say that we choose the domain `mesos.sfo.widgets.com` to expose our services.

Domain Hierarchy
```
 .com
 |
 +--.widgets.com
    |
    +--sfo.widgets.com
       |
       +--mesos.sfo.widgets.com
 ```

2. We need to configure Mesos-DNS to use the domain `mesos.sfo.widgets.com`, as opposed to `.mesos`. The fields in the configuration file to modify are the following:

    `domain` - Set this the domain that Mesos-DNS will be available on. In this example, it should be `mesos.sfo.widgets.com`

    `SOAMname` - This is the name of the instance running Mesos-DNS. Each of your Mesos-DNS instances should have a separate one. For example: `mesosdns1.sfo.widgets.com`

    `listener` (optional) - Potentially, you may want to collocate the Mesos-DNS instance next to a BIND instance. In this case, you may add a special IP for the Mesos-DNS instance, so you don't need another machine.

    `externalon` (optional) - On this Mesos-DNS instance, the external usage of nameserver should be disabled to prevent abuse, or loops

    `httpon` (optional) - On this Mesos-DNS instance, the HTTP should be disabled

3. We need to configure the nameserver with the proper glue records to expose the Mesos-DNS servers. This is all you need to add. This delegates the `mesos.sfo.widgets.com` domain to the nameserver `mesosdns1.sfo.widgets.com`. The name `mesosdns1` should match with the `SOAMname` that we set in the Mesos-DNS configuration.
```
mesos     IN  NS  mesosdns1
mesosdns1 IN  A   33.33.33.2
```

4. Using it:



#### Domain Heirarchy

