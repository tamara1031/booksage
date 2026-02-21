from abc import ABC

from .ports import IVectorStore


class BaseVectorStore(IVectorStore, ABC):
    """Base generic vector store wrapper if common functionality is needed."""

    def __init__(self, collection_name: str, dimension: int):
        self.collection_name = collection_name
        self.dimension = dimension
