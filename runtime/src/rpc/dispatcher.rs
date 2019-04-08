//! RPC dispatcher.
use std::collections::HashMap;

use failure::Fallible;
use serde::{de::DeserializeOwned, Serialize};
use serde_cbor;

use super::{
    context::Context,
    types::{Body, Request, Response},
};

/// Dispatch error.
#[derive(Debug, Fail)]
enum DispatchError {
    #[fail(display = "method not found: {}", method)]
    MethodNotFound { method: String },
}

/// Descriptor of a RPC API method.
#[derive(Clone, Debug)]
pub struct MethodDescriptor {
    /// Method name.
    pub name: String,
}

/// Handler for a RPC method.
pub trait MethodHandler<Rq, Rsp> {
    /// Invoke the method implementation and return a response.
    fn handle(&self, request: &Rq, ctx: &mut Context) -> Fallible<Rsp>;
}

impl<Rq, Rsp, F> MethodHandler<Rq, Rsp> for F
where
    Rq: 'static,
    Rsp: 'static,
    F: Fn(&Rq, &mut Context) -> Fallible<Rsp> + 'static,
{
    fn handle(&self, request: &Rq, ctx: &mut Context) -> Fallible<Rsp> {
        (*self)(&request, ctx)
    }
}

/// Dispatcher for a RPC method.
pub trait MethodHandlerDispatch {
    /// Get method descriptor.
    fn get_descriptor(&self) -> &MethodDescriptor;

    /// Dispatch request.
    fn dispatch(&self, request: Request, ctx: &mut Context) -> Fallible<Response>;
}

struct MethodHandlerDispatchImpl<Rq, Rsp> {
    /// Method descriptor.
    descriptor: MethodDescriptor,
    /// Method handler.
    handler: Box<MethodHandler<Rq, Rsp>>,
}

impl<Rq, Rsp> MethodHandlerDispatch for MethodHandlerDispatchImpl<Rq, Rsp>
where
    Rq: DeserializeOwned + 'static,
    Rsp: Serialize + 'static,
{
    fn get_descriptor(&self) -> &MethodDescriptor {
        &self.descriptor
    }

    fn dispatch(&self, request: Request, ctx: &mut Context) -> Fallible<Response> {
        let request = serde_cbor::from_value(request.args)?;
        let response = self.handler.handle(&request, ctx)?;

        Ok(Response {
            body: Body::Success(serde_cbor::to_value(response)?),
        })
    }
}

/// RPC method dispatcher implementation.
pub struct Method {
    /// Method dispatcher.
    dispatcher: Box<MethodHandlerDispatch>,
}

impl Method {
    /// Create a new enclave method descriptor.
    pub fn new<Rq, Rsp, Handler>(method: MethodDescriptor, handler: Handler) -> Self
    where
        Rq: DeserializeOwned + 'static,
        Rsp: Serialize + 'static,
        Handler: MethodHandler<Rq, Rsp> + 'static,
    {
        Method {
            dispatcher: Box::new(MethodHandlerDispatchImpl {
                descriptor: method,
                handler: Box::new(handler),
            }),
        }
    }

    /// Return method name.
    pub fn get_name(&self) -> &String {
        &self.dispatcher.get_descriptor().name
    }

    /// Dispatch arequest.
    pub fn dispatch(&self, request: Request, ctx: &mut Context) -> Fallible<Response> {
        self.dispatcher.dispatch(request, ctx)
    }
}

/// RPC call dispatcher.
pub struct Dispatcher {
    /// Registered RPC methods.
    methods: HashMap<String, Method>,
}

impl Dispatcher {
    /// Create a new RPC method dispatcher.
    pub fn new() -> Self {
        Self {
            methods: HashMap::new(),
        }
    }

    /// Register a new method in the dispatcher.
    pub fn add_method(&mut self, method: Method) {
        self.methods.insert(method.get_name().clone(), method);
    }

    /// Dispatch request.
    pub fn dispatch(&self, request: Request, mut ctx: Context) -> Response {
        match self.dispatch_fallible(request, &mut ctx) {
            Ok(response) => response,
            Err(error) => Response {
                body: Body::Error(format!("{}", error)),
            },
        }
    }

    fn dispatch_fallible(&self, request: Request, ctx: &mut Context) -> Fallible<Response> {
        match self.methods.get(&request.method) {
            Some(dispatcher) => dispatcher.dispatch(request, ctx),
            None => Err(DispatchError::MethodNotFound {
                method: request.method,
            }
            .into()),
        }
    }
}