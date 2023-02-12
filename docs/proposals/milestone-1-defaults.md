# Milestone-1 defaults

### Terminology
Proposed terminology, one of these two terms need to replace `--hub-router`, either `--discovery-node`, 
`--relay-node` (or something else):

- _Relay Node_: is for hard NAT traversal (symmetric NAT only) The hub in a hub/spoke model.
- _Discovery Node_: ICE/NAT traversal discovery.

### Default Deployments

1. The SaaS demo will NOT provide a discovery or relay node. While the functionality is there, 
the implementation relies on Wireguard state from the Discovery Node (e.g. hub-router). 
A user can add their own discovery/relay node offered via an image or starting the agent as that node. 
Alternatively, they can kick the tires using option #2 in this list.In the long run, NAT-T discovery 
is a vital component to the user experience and should be included in an organization be default. 
The mechanism to get there will be generic, Anil is currently on point there but the more eyes/ideas 
the better there. It is not a trivial task and will need extensive testing in diverse environments 
that we will all need to help vet.In the short term using Wireguard state works very well and can be 
leveraged for demo environments to accomplish ICE like functionality. Making the discovery process 
driver agnostic (to facilitate future drivers such as QUIC) and not a peer in the mesh is the goal. 
Discovery node being provisioned from the stack per/organization will be a milestone target in the future.


2. To address user experience concerns regarding not providing a discovery or relay node, we will have a 
demo organization containing provisioned discovery and relay functionality (virtually identical to 
the current environment Stephen has been testing in) that a user can join when they authenticate 
with github OAUTH. One option could be an invitation to the demo organization in the one-time login redirect page. 
This would allow users to experience a multi-cloud connectivity deployment and join any of their own nodes. 
There should be a disclaimer that the environment will have other invited users there that will they 
will be peering with. This environment will represent the ease of deployment and node onboarding 
we are targeting with Apex.
