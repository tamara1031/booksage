from booksage.infrastructure import MilvusVectorStore


def test_milvus_store_mock():
    store = MilvusVectorStore(collection_name="test", dimension=1536)
    store.connect()
    # verify add_chunks doesn't crash on mock
    store.add_chunks([])
    res = store.search("test query", {}, top_k=5)
    assert isinstance(res, list)
    assert len(res) == 0
