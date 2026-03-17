---
title: dependency-injection
date: 2026-02-10
tags: [concept, acme, dotnet]
---

# dependency-injection

## What It Is
A pattern where a class receives its dependencies from outside rather than creating them itself. The container (or caller) wires everything together.

## How We Use It
In .NET 10 we register services in Program.cs using the built-in DI container. Constructor injection is preferred. We never use service locator pattern.

## Examples
```csharp
services.AddScoped<IOrderRepository, SqlOrderRepository>();
services.AddScoped<IOrderService, OrderService>();
```

## Links
- [[use-dapper-not-efcore]]
