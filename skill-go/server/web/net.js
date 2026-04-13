// ---------------------------------------------------------------------------
// net.js — Unified network client for skill-go
// ---------------------------------------------------------------------------

/**
 * NetError represents a structured network error.
 */
export class NetError extends Error {
  constructor(code, message, httpStatus) {
    super(message);
    this.name = 'NetError';
    this.code = code;          // e.g. "bad_request", "not_found"
    this.httpStatus = httpStatus; // e.g. 400, 404
    this.isNetworkError = httpStatus === 0;
  }
}

/**
 * Subscription manages an SSE stream lifecycle.
 */
export class Subscription {
  constructor(path, handler, netClient) {
    this._path = path;
    this._handler = handler;
    this._netClient = netClient;
    this._eventSource = null;
    this._closed = false;
    this._reconnectTimer = null;
    this._onReconnect = null;
    this._connect();
  }

  _connect() {
    if (this._closed) return;
    this._eventSource = new EventSource(this._path);

    this._eventSource.onopen = () => {
      if (this._onReconnect) this._onReconnect('connected');
    };

    this._eventSource.onmessage = (msg) => {
      try {
        const event = JSON.parse(msg.data);
        this._handler(event);
      } catch (e) {
        // ignore parse errors
      }
    };

    this._eventSource.onerror = () => {
      if (this._onReconnect) this._onReconnect('reconnecting');
      this._cleanup();
      if (!this._closed) {
        this._reconnectTimer = setTimeout(() => this._connect(), 3000);
      }
    };
  }

  /** Called on reconnect or initial connection. callback(status: 'connected'|'reconnecting') */
  set onReconnect(cb) {
    this._onReconnect = cb;
  }

  close() {
    this._closed = true;
    this._cleanup();
    if (this._reconnectTimer) {
      clearTimeout(this._reconnectTimer);
      this._reconnectTimer = null;
    }
  }

  _cleanup() {
    if (this._eventSource) {
      this._eventSource.close();
      this._eventSource = null;
    }
  }
}

/**
 * NetClient provides a unified interface for all API communication.
 *
 * Usage:
 *   import { netClient } from '/net.js';
 *   const spells = await netClient.get('/api/spells');
 *   const result = await netClient.post('/api/cast', { spellID: 42833, targetIDs: [3] });
 */
export class NetClient {
  constructor() {
    this._errorHandlers = [];
  }

  // --- REST methods ---

  async get(path, params) {
    const url = params ? this._buildURL(path, params) : path;
    return this._request('GET', url);
  }

  async post(path, body) {
    return this._request('POST', path, body);
  }

  async put(path, body) {
    return this._request('PUT', path, body);
  }

  async del(path) {
    return this._request('DELETE', path);
  }

  // --- SSE subscription ---

  /**
   * Subscribe to an SSE stream.
   * @param {string} path - SSE endpoint path
   * @param {function} handler - callback for each event
   * @returns {Subscription}
   */
  subscribe(path, handler) {
    return new Subscription(path, handler, this);
  }

  // --- Error handling ---

  /**
   * Register a global error handler.
   * @param {function(error: NetError)} handler
   */
  onError(handler) {
    this._errorHandlers.push(handler);
  }

  // --- Internal ---

  async _request(method, url, body) {
    const options = {
      method,
      headers: { 'Content-Type': 'application/json' },
    };
    if (body !== undefined) {
      options.body = JSON.stringify(body);
    }

    let resp;
    try {
      resp = await fetch(url, options);
    } catch (err) {
      const netErr = new NetError('network_error', err.message, 0);
      this._notifyError(netErr);
      throw netErr;
    }

    if (!resp.ok) {
      let code = 'http_error';
      let message = `${method} ${url}: ${resp.status}`;
      try {
        const errBody = await resp.json();
        if (errBody.error) {
          code = errBody.error.code || code;
          message = errBody.error.message || message;
        }
      } catch (e) {
        // response body not JSON, use default message
      }
      const netErr = new NetError(code, message, resp.status);
      this._notifyError(netErr);
      throw netErr;
    }

    return resp.json();
  }

  _buildURL(path, params) {
    const qs = Object.entries(params)
      .filter(([, v]) => v !== undefined && v !== null && v !== '')
      .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(v)}`)
      .join('&');
    return qs ? `${path}?${qs}` : path;
  }

  _notifyError(error) {
    for (const handler of this._errorHandlers) {
      try {
        handler(error);
      } catch (e) {
        // prevent handler errors from propagating
      }
    }
  }
}

// Global singleton instance
export const netClient = new NetClient();
