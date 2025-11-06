<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Application\UseCases\CreateUser;

readonly class CreateUserResult
{
    public function __construct(
        public int $id
    )
    {
    }


}
