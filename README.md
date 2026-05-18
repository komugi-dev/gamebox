# gamebox
API and lilbrary supporting turn based games.

# Design Philosophy & Architecture
This project was built with a dual purpose: to focus purely on game engines and to study language-agnostic AI players. To achieve this, the project is built upon the following core principles:

Design-First Approach: Engineered following a rigorous top-down methodology (Purpose -> Architecture -> Protocol -> Skeleton) rather than rushing implementation.

Robust Concurrency & Clean Architecture: The underlying skeleton securely handles game concurrency and protocol bridging, while keeping the domain logic strictly isolated from the transport layer.

Language-Agnostic Protocol: The network layer is entirely decoupled from the game logic. By relying on standard WebSockets and a clean JSON message flow, the engine is agnostic to the client. AI agents can be written in Python, Rust, or Node.js and seamlessly connect to this Go server.

Modern Tooling: Developed adopting an AI-assisted workflow, utilizing AI as a pair-programmer and architectural sounding board to iterate on design choices.
