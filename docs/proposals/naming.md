# Enhancement Title

> [Issue #441](https://github.com/redhat-et/issues/441)

## Summary

Proposal for using consistent naming for the major components of the Apex stack

## Proposal

Currently we are are using multiple names for different components across our documentation and code, such as

* Apex Stack is referred as Apex Service, Apex ApiServer, Apex Controller, Apex Control Plane, Apex SaaS
* Apex Agent is referred as hub-router, relay, agent, discovery node

It would be really helpful to write a coherent documentation and code if we can zero down on the naming of these components.

Here is my initial proposal to start the discussion

* Apex Service: It's an umbrella term to refer to the entire stack, that contains following individual components (Named as is).
  * Apex ApiServer
  * Apex Frontend
  * Apex Database
  * Apex Ipam
  * Apex ApiProxy
  * Apex backend-cli
  * Apex backend-web
* Apex Agent: Agent running on each node, just doing Agent's job.
  * Binary Naming: We need to go one more level down to clarify the binary naming as we are planning to separate the relay functionality out of agent.
    * apexd: Agent binary (running as a daemon) that does the magic of connectivity.
    * apex: A command line utility to do read/write operation through Apexd
* Apex ICE: A node running the discovery and relay function.
  * Binary Naming:
    * apexiced: Binary (running as a daemon) that does the magic of discovery and relay.
    * apexice: A command line utility to do read/write operation through Apexiced
  
> **Warning**
> This proposal needs to updated when [Moving Repository Discussion](https://github.com/redhat-et/apex/pull/440) concludes.

## Alternatives Considered

I assume other alternative will come up during the discussion and all but the winner will end up in this section.

## Conclusion
