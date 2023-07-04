use axum::extract::{Path, State};
use axum::{http::StatusCode, routing::get, Router};
use clap::Parser;
use hyperlocal::UnixServerExt;
use kbs_protocol::{KbsProtocolWrapper, KbsRequest};
use std::convert::From;
use std::path::PathBuf;
use tokio::signal;
use tower_http::trace::{self, TraceLayer};
use tracing::{error, info, Level};

/// Secure Key Release API
#[derive(Parser, Debug, Clone)]
#[command(author, version, about, long_about = None)]
struct Config {
    /// URI of the KBS to query
    #[arg(short, long, default_value = "http://127.0.0.1:8080")]
    kbs_url: String,

    /// unix domain socket to listen on
    #[arg(
        short,
        long,
        default_value = "/run/confidential-containers/skr-api.sock"
    )]
    socket_path: String,
}

async fn shutdown_signal(path: &PathBuf) {
    let ctrl_c = async {
        signal::ctrl_c()
            .await
            .expect("failed to install Ctrl+C handler");
    };

    let terminate = async {
        signal::unix::signal(signal::unix::SignalKind::terminate())
            .expect("failed to install signal handler")
            .recv()
            .await;
    };

    tokio::select! {
        _ = ctrl_c => {},
        _ = terminate => {},
    }

    info!("signal received, starting graceful shutdown");
    let _ = tokio::fs::remove_file(&path).await;
}

#[derive(serde::Deserialize)]
struct Resource {
    repository_name: String,
    r#type: String,
    tag: String,
}

async fn request_key(
    State(config): State<Config>,
    Path(resource): Path<Resource>,
) -> Result<Vec<u8>, StatusCode> {
    let mut kbs_protocol_wrapper = KbsProtocolWrapper::new().map_err(|e| {
        error!("failed to create KbsProtocolWrapper: {:?}", e);
        StatusCode::INTERNAL_SERVER_ERROR
    })?;
    let resource_url = format!(
        "{}/kbs/v0/resource/{}/{}/{}",
        &config.kbs_url, &resource.repository_name, &resource.r#type, &resource.tag
    );
    let bytes = kbs_protocol_wrapper
        .http_get(resource_url)
        .await
        .map_err(|e| {
            error!("failed to get resource: {:?}", e);
            StatusCode::BAD_REQUEST
        })?;
    Ok(bytes)
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    tracing_subscriber::fmt().init();
    let http_trace_layer = TraceLayer::new_for_http()
        .make_span_with(trace::DefaultMakeSpan::new().level(Level::INFO))
        .on_response(trace::DefaultOnResponse::new().level(Level::INFO));

    let config = Config::parse();

    let path = PathBuf::try_from(&config.socket_path)?;

    let app = Router::new()
        .route("/getresource/:repository_name/:type/:tag", get(request_key))
        .route("/health", get(|| async { "OK" }))
        .with_state(config)
        .layer(http_trace_layer);

    axum::Server::bind_unix(&path)?
        .serve(app.into_make_service())
        .with_graceful_shutdown(shutdown_signal(&path))
        .await?;

    Ok(())
}
