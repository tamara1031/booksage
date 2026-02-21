from booksage.domain import Chunk

from .store_base import BaseVectorStore


class MilvusVectorStore(BaseVectorStore):
    def __init__(self, collection_name: str, dimension: int, uri: str = "http://localhost:19530"):
        super().__init__(collection_name, dimension)
        self.uri = uri
        self._client = None

    def connect(self):
        """Mockable connection logic to Milvus via PyMilvus."""
        # from pymilvus import connections
        # connections.connect("default", uri=self.uri)
        pass

    def add_chunks(self, chunks: list[Chunk]) -> None:
        """Insert chunks into Milvus collection. Separates metadata and dense vectors."""
        # e.g., collection.insert(...)
        pass

    def search(self, query: str, filters: dict, top_k: int) -> list[Chunk]:
        """Search Milvus using query vector and scalar filters."""
        # Return empty for now as a mock
        return []
