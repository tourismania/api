<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Application\UseCases\GetMe;

readonly class GetMeQuery
{
    /**
     * @param string[] $roles
     */
    public function __construct(
        public int $id,
        public string $email,
        public string $phone,
        public string $firstName,
        public string $lastName,
        public array $roles,
    ) {
    }
}
