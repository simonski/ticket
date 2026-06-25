DATABASE CHANGE

I want to refine a new story that has a high quality title, descriptiojn, ac etc.   

Frequently we introduce change to the tk system itself that incurs a database schema change.  

What I want to do is use something other than "a new column" and consider how to embed more into the schema we have - or, amend hte schema such that we dont have to amnd it very often in the future - that is, lower the risk of schema change.  

I think this is achieved using e.g. jsonb style or jsonl style - or plain old json style columns in the database to hold key/pairs or object/entities.

Reason with me and design a more effective schema system.  

Rules
- lowers the likelihood of schema change in the future
- we can perform schema change this time - and it must work against the existing db which I will make a  reference copy available to you locally.  
- reinforce the migration logic such that it retains a backup of the prior so as to de-risk a broken migration - note, this is just the normal migration logic that I am asking you to reinforce

---

Once we have reasoned the design to a reasonable point, I want you to create a high-quality tk ticket using tk new.  I expect it will have a title, then a comprehensive description and acceptance criteria.

I woud like to see upgraded documentation on the feature branch first along with design documents and diagrams to ensure the design is correct, before we implement any code.


# git

use an epic feature branch and make each story int he epic branch from the epic feature branch, each with their own PR against the epic.   Once all stories are complete then the epic can be PRed against the main branch such that the entire feature is merged to main only once it is complete.

refine the above with me until it is ready then generate an epic and sub-stories to work on
