cmake_minimum_required(VERSION 3.16)

project(MatchingEngine VERSION 0.1.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

option(MATCHING_ENGINE_BUILD_SMOKE "Build and run a smoke executable" ON)

find_package(Threads REQUIRED)

# Core matching engine (static lib from existing LOB code)
add_library(matching_engine_core STATIC
    Limit_Order_Book/Book.cpp
    Limit_Order_Book/Limit.cpp
    Limit_Order_Book/Order.cpp
)

set_target_properties(matching_engine_core PROPERTIES POSITION_INDEPENDENT_CODE ON)

target_include_directories(matching_engine_core
    PUBLIC
        ${CMAKE_CURRENT_SOURCE_DIR}/include
        ${CMAKE_CURRENT_SOURCE_DIR}/Limit_Order_Book
)

target_compile_options(matching_engine_core PRIVATE -Wall -Wextra -Wpedantic)

# C API shared library — this is what Go links to via CGO
add_library(matching_engine_c_api SHARED
    src/engine_c_api.cpp
)

target_link_libraries(matching_engine_c_api PRIVATE matching_engine_core)
target_include_directories(matching_engine_c_api
    PUBLIC
        ${CMAKE_CURRENT_SOURCE_DIR}/include
        ${CMAKE_CURRENT_SOURCE_DIR}/Limit_Order_Book
)

target_compile_options(matching_engine_c_api PRIVATE -Wall -Wextra -Wpedantic)

# Smoke test
if(MATCHING_ENGINE_BUILD_SMOKE)
    add_executable(matching_engine_smoke tests/smoke_main.cpp)
    target_link_libraries(matching_engine_smoke PRIVATE matching_engine_core)
    target_include_directories(matching_engine_smoke PRIVATE
        ${CMAKE_CURRENT_SOURCE_DIR}/Limit_Order_Book
    )
endif()

enable_testing()

if(TARGET matching_engine_smoke)
    add_test(NAME matching_engine_smoke_test COMMAND matching_engine_smoke)
endif()
