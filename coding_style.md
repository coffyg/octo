# Coffyg Go Coding Style Guide

This document defines the "coffyg style" - our opinionated approach to writing safe, performant, and maintainable Go code.

## Safety

> "The rules act like the seat-belt in your car: initially they are perhaps a little uncomfortable,
> but after a while their use becomes second-nature and not using them becomes unimaginable." —
> Gerard J. Holzmann

The Power of Ten — Rules for Developing Safety Critical Code will change the way you code forever. To expand:

- Use **only very simple, explicit control flow** for clarity. **Use recursion sparingly** to ensure that all executions that should be bounded are bounded. Use **only a minimum of excellent abstractions** but only if they make the best sense of the domain. Abstractions are never zero cost. Every abstraction introduces the risk of a leaky abstraction.

- **Put a limit on everything** because, in reality, this is what we expect—everything has a limit. For example, all loops and all queues must have a fixed upper bound to prevent infinite loops or tail latency spikes. This follows the ["fail-fast"](https://en.wikipedia.org/wiki/Fail-fast) principle so that violations are detected sooner rather than later.

- Use explicitly-sized types (like `int64`, `int32` instead of plain `int`) where the size matters. This provides clearer intentions and prevents architecture-specific behaviors.

- **Assertions detect programmer errors.** Unlike operating errors, which are expected and which must be handled, assertion failures are unexpected. Assertions downgrade catastrophic correctness bugs into more tractable bugs.

  - **Assert all function preconditions and postconditions.** A function must not operate blindly on data it has not checked. The purpose of a function is to increase the probability that a program is correct. Use `if err != nil` blocks at function entry points to validate inputs.

  - **Test both edges.** For every property you want to enforce, try to find at least two different code paths where validation can be added. For example, validate data right before writing it to a file, and also immediately after reading from a file.

  - Split compound conditionals: prefer `if a { if b { ... } }` over `if a && b { ... }`. The former is simpler to debug, and provides more precise information about which condition failed.

  - Always check and test error conditions. Never silently ignore errors.

- Declare variables at the **smallest possible scope**, and **minimize the number of variables in scope**, to reduce the probability that variables are misused.

- Restrict the length of function bodies to reduce the probability of poorly structured code. We enforce a **hard limit of 70 lines per function**.

  Splitting code into functions requires taste. There are many ways to cut a wall of code into chunks of 70 lines, but only a few splits will feel right. Some rules of thumb:

  * Good function shape is often the inverse of an hourglass: a few parameters, a simple return type, and a lot of meaty logic between the braces.
  * Centralize control flow. When splitting a large function, try to keep all switch/if statements in the "parent" function, and move non-branchy logic fragments to helper functions. Divide responsibility. All control flow should be handled by _one_ function, the rest shouldn't care about control flow at all.
  * Similarly, centralize state manipulation. Let the parent function keep all relevant state in local variables, and use helpers to compute what needs to change, rather than applying the change directly. Keep leaf functions pure.

- Appreciate, from day one, **all compiler warnings at the compiler's strictest setting**. Use `go vet` and linters consistently.

- Whenever your program has to interact with external entities, **don't do things directly in reaction to external events**. Instead, your program should run at its own pace. Not only does this make your program safer by keeping the control flow of your program under your control, it also improves performance (you get to batch, instead of context switching on every event). Additionally, this makes it easier to maintain bounds on work done per time period.

Beyond these rules:

- Compound conditions that evaluate multiple booleans make it difficult for the reader to verify that all cases are handled. Split compound conditions into simple conditions using nested `if/else` branches. Split complex `else if` chains into `else { if { } }` trees. This makes the branches and cases clear.

- All errors must be handled. An [analysis of production failures in distributed data-intensive systems](https://www.usenix.org/system/files/conference/osdi14/osdi14-paper-yuan.pdf) found that the majority of catastrophic failures could have been prevented by simple testing of error handling code.

> "Specifically, we found that almost all (92%) of the catastrophic system failures are the result of incorrect handling of non-fatal errors explicitly signaled in software."

- **Always motivate, always say why**. Never forget to say why. Because if you explain the rationale for a decision, it not only increases the hearer's understanding, and makes them more likely to adhere or comply, but it also shares criteria with them with which to evaluate the decision and its importance.

- **Explicitly pass options to library functions at the call site, instead of relying on the defaults**. For example, prefer explicitly setting timeouts rather than using defaults. This improves readability and avoids latent, potentially catastrophic bugs in case the library ever changes its defaults.

## Performance

> "The lack of back-of-the-envelope performance sketches is the root of all evil." — Rivacindela Hudsoni

- Think about performance from the outset, from the beginning. **The best time to solve performance, to get the huge 1000x wins, is in the design phase, which is precisely when we can't measure or profile.** It's also typically harder to fix a system after implementation and profiling, and the gains are less. So you have to have mechanical sympathy.

- **Perform back-of-the-envelope sketches with respect to the four resources (network, disk, memory, CPU) and their two main characteristics (bandwidth, latency).** Sketches are cheap. Use sketches to be "roughly right" and land within 90% of the global maximum.

- Optimize for the slowest resources first (network, disk, memory, CPU) in that order, after compensating for the frequency of usage, because faster resources may be used many times more. For example, a memory cache miss may be as expensive as a disk fsync, if it happens many times more.

- Distinguish between the control plane and data plane. A clear delineation between control plane and data plane through the use of batching enables a high level of safety without losing performance.

- Amortize network, disk, memory and CPU costs by batching accesses.

- Be explicit. Minimize dependence on the compiler to do the right thing for you.

### Naming Things

- **Get the nouns and verbs just right.** Great names are the essence of great code, they capture what a thing is or does, and provide a crisp, intuitive mental model. They show that you understand the domain. Take time to find the perfect name, to find nouns and verbs that work together, so that the whole is greater than the sum of its parts.

- Use `camelCase` for unexported function, variable, and file names and `PascalCase` for exported ones. Follow Go's conventions for naming, where the first letter's case determines visibility.

- Do not abbreviate variable names, unless the variable is a primitive integer type used as an argument to a sort function or matrix calculation. Use proper capitalization for acronyms (`HTTPHandler`, not `HttpHandler`).

- Add units or qualifiers to variable names, and put the units or qualifiers last, sorted by descending significance, so that the variable starts with the most significant word, and ends with the least significant word. For example, `latencyMsMax` rather than `maxLatencyMs`. This will then line up nicely when `latencyMsMin` is added, as well as group all variables that relate to latency.

- When choosing related names, try hard to find names with the same number of characters so that related variables all line up in the source. For example, as arguments to a function, `source` and `target` are better than `src` and `dest` because they have the same length and any related variables such as `sourceOffset` and `targetOffset` will all line up in calculations and slices. This makes the code symmetrical, with clean blocks that are easier for the eye to parse and for the reader to check.

- When a single function calls out to a helper function or callback, prefix the name of the helper function with the name of the calling function to show the call history. For example, `readSector()` and `readSectorCallback()`.

- _Order_ matters for readability (even if it doesn't affect semantics). On the first read, a file is read top-down, so put important things near the top. The `main` function goes first.

- Don't overload names with multiple meanings that are context-dependent. This can lead to confusion and bugs.

- **Write descriptive commit messages** that inform and delight the reader, because your commit messages are being read.

## Comments

- **Use comments with parsimony.** Most code should be self-explanatory through good naming, clear structure, and intuitive design. Comments should be reserved for complex logic that cannot be simplified further.

- **Write comments as if speaking to a human colleague.** Use natural language that explains the "why" behind the code, not the "what" that is already visible in the code itself.

- **Focus comments on intent and rationale.** Explain design decisions, tradeoffs considered, or potential edge cases rather than restating what the code does.

- **Keep comments updated.** Outdated comments are worse than no comments at all. When changing code, review and update the associated comments.

- Comments should be complete sentences with proper punctuation. Comments after the end of a line _can_ be phrases with no punctuation.

## Dependencies

Follow Go's preference for a "batteries included" standard library. Use third-party dependencies only when necessary and vet them carefully. Too many dependencies inevitably lead to supply chain attacks, safety and performance risk, and slower build times.

In the Coffyg style, we use a small, carefully curated set of dependencies:
- `zerolog` for structured logging
- `github.com/pkg/errors` for error handling with stack traces
- `go-playground/form` for form binding

## Tooling

A small standardized toolbox is simpler to operate than an array of specialized instruments each with a dedicated manual. Our primary tool is Go. It may not be the best for everything, but it's good enough for most things.

> "The right tool for the job is often the tool you are already using—adding new tools has a higher cost than many people appreciate" — John Carmack

## Style By The Numbers

- Run `gofmt` or `go fmt` on all Go files.

- Use 4 spaces of indentation, rather than tabs or 2 spaces, for better readability.

- Hard limit all line lengths to at most 100 columns. Let your editor help you by setting a column ruler.

## Error Handling

Go's explicit error handling is a feature, not a bug. Embrace it:

- Always check error returns and handle them appropriately
- Use `github.com/pkg/errors` for error wrapping and stack traces
- Keep error messages consistent, informative, and actionable
- Log errors with appropriate context

## The Coffyg Way

In the coffyg style, we value:
- Type-safe code with generics
- Middleware-based architecture
- Explicit configuration over implicit defaults
- Recovery from panics in production code
- Comprehensive benchmarking
- Context propagation and proper request lifecycle management

---

This coding style guide is inspired by [TigerBeetle's Tiger Style](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md), adapted for Go and the specific needs of the Coffyg ecosystem.