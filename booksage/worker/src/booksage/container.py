from booksage.chunking.ports import IChunker
from booksage.etl.ports import IDocumentParser
from booksage.generation.ports import IGenerationAgent
from booksage.infrastructure.ports import IVectorStore
from booksage.retrieval.ports import IRetrievalEngine


class Container:
    """Dependency Injection Container for the BookSage Application."""

    def __init__(self):
        self.pdf_parser: IDocumentParser = None
        self.epub_parser: IDocumentParser = None
        self.chunker: IChunker = None
        self.vector_store: IVectorStore = None
        self.engines: list[IRetrievalEngine] = []
        self.retriever: IRetrievalEngine = None
        self.agent: IGenerationAgent = None

    def init_resources(self):
        """Initialize all concrete dependencies here."""
        from booksage.chunking import FreeChunker
        from booksage.etl import EpubParser, PyMuPDFParser
        from booksage.generation import AgenticGenerator
        from booksage.infrastructure import MilvusVectorStore
        from booksage.retrieval import (
            ColBERTV2Engine,
            FusionRetriever,
            LightRAGEngine,
            RAPTOREngine,
        )

        self.pdf_parser = PyMuPDFParser()
        self.epub_parser = EpubParser()
        self.chunker = FreeChunker(chunk_size=1000, chunk_overlap=200)
        self.vector_store = MilvusVectorStore(collection_name="booksage_index", dimension=1536)

        self.engines = [LightRAGEngine(), RAPTOREngine(), ColBERTV2Engine()]
        self.retriever = FusionRetriever(engines=self.engines)
        self.agent = AgenticGenerator(retriever=self.retriever)


container = Container()
container.init_resources()
