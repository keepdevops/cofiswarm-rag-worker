"""Attach the synchronous rag-worker daemon to the NATS observer bus via the shared
cofiswarm-observer-sdk Python ServiceComponent.

The worker has no asyncio loop (it's threading + a blocking drain loop), so the async component
runs on a dedicated background thread; the main loop stays synchronous and drives goodbye via
run_coroutine_threadsafe on shutdown.

Default-off: enabled only when COFISWARM_NATS_URL is set. The SDK is imported lazily, so the
worker runs identically — and without cofiswarm-observer-sdk installed — when the bus is off.
"""
from __future__ import annotations

import asyncio
import logging
import os
import threading

logger = logging.getLogger("rag-worker.observer")


class BusPresence:
    """Runs an async ServiceComponent on a background thread for a synchronous host."""

    def __init__(self) -> None:
        self._loop: asyncio.AbstractEventLoop | None = None
        self._thread: threading.Thread | None = None
        self._comp = None
        self._nc = None

    def start(self) -> None:
        """Announce presence on a background asyncio thread. No-op unless COFISWARM_NATS_URL is
        set; never imports the SDK or starts a thread when disabled."""
        url = os.environ.get("COFISWARM_NATS_URL")
        if not url:
            logger.info("observer: COFISWARM_NATS_URL unset; bus attach disabled")
            return
        try:
            from cofiswarm_observer import ServiceComponent, contract
        except ImportError as exc:  # loud: asked for the bus but the client isn't installed
            logger.error("observer: COFISWARM_NATS_URL set but cofiswarm-observer-sdk missing: %s", exc)
            return

        ready = threading.Event()

        def run() -> None:
            loop = asyncio.new_event_loop()
            self._loop = loop
            asyncio.set_event_loop(loop)

            async def info(_req: dict) -> dict:
                return {"component": "rag-worker"}

            async def health(_req: dict) -> dict:
                return {"status": "ok"}

            async def boot() -> None:
                try:
                    self._nc = await ServiceComponent.connect(url, "cofiswarm-rag-worker")
                except Exception as exc:  # loud: never silently run detached from the bus
                    logger.error("observer: NATS connect %s failed: %s", url, exc)
                    return
                routes = {
                    f"{contract.PREFIX}.rag-worker.info": info,
                    f"{contract.PREFIX}.rag-worker.health": health,
                }
                self._comp = ServiceComponent(self._nc, "rag-worker", "rag-worker", routes)
                await self._comp.start()
                logger.info("observer: rag-worker announced on %s (.rag-worker.info/.health)", url)

            loop.run_until_complete(boot())
            ready.set()
            loop.run_forever()
            loop.close()

        self._thread = threading.Thread(target=run, name="observer-bus", daemon=True)
        self._thread.start()
        ready.wait(timeout=10)

    def stop(self) -> None:
        """Say goodbye (flip offline) and tear down the background loop/thread."""
        if self._loop is None:
            return

        async def teardown() -> None:
            if self._comp is not None:
                await self._comp.shutdown()  # goodbye -> offline
            if self._nc is not None:
                await self._nc.close()

        try:
            asyncio.run_coroutine_threadsafe(teardown(), self._loop).result(timeout=5)
        except Exception as exc:
            logger.error("observer: shutdown failed: %s", exc)
        self._loop.call_soon_threadsafe(self._loop.stop)
        if self._thread is not None:
            self._thread.join(timeout=5)
