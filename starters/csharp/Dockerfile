FROM mcr.microsoft.com/dotnet/sdk:9.0 AS builder

WORKDIR /app
COPY acb-starter-csharp.csproj .
RUN dotnet restore
COPY Program.cs Grid.cs .
RUN dotnet publish -c Release -o /out

FROM mcr.microsoft.com/dotnet/runtime:9.0-alpine

WORKDIR /app
COPY --from=builder /out .

ENV BOT_PORT=8080
ENV BOT_SECRET=""

EXPOSE 8080

CMD ["dotnet", "acb-starter-csharp.dll"]
