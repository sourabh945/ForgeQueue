# ForgeQueue
A distributed image processing job queue built from the ground up — Go orchestration, Python workers, Unix IPC, Redis-backed job management, and webhook delivery.

##File Structure: 
ForgeQueue/
├── manager/          # Go Orchestrator (Go modules live here)
├── workers/          # Python Worker scripts
├── api/              # Node.js API (Express)
├── shared/           # Shared volume for images (local testing)
├── .gitignore
└── README.md         # Document as you build
